package application

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sergey-suslov/awesome-chat/common"
	"github.com/sergey-suslov/awesome-chat/common/types"
	"github.com/sergey-suslov/awesome-chat/packages/messenger/shared"
	"go.uber.org/zap"
)

type UserConnector interface {
	ConnectToChat(cc shared.ClientConnection, id string, pub []byte) error
	Disconnect(cc shared.ClientConnection, id string) error
	SubscribeUserToMessages(cc shared.ClientConnection, id string) (common.TermChan, error)
	SubscribeUserToChatMessages(cc shared.ClientConnection, id string) (common.TermChan, error)
}

type Messenger interface {
	SendMessage(userId string, data []byte) error
}

type UserConnection struct {
	conn          *websocket.Conn
	logger        *zap.SugaredLogger
	sendChan      chan types.Message
	userConnector UserConnector
	messenger     Messenger
	Id            string
	term          sync.WaitGroup
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 5 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

func NewUserConnection(userConnection *websocket.Conn, logger *zap.SugaredLogger, send chan types.Message, userConnector UserConnector, messenger Messenger) *UserConnection {
	return &UserConnection{Id: string(uuid.New().String()), conn: userConnection, logger: logger, sendChan: send, userConnector: userConnector, messenger: messenger}
}

func (uc *UserConnection) Run() error {
	termUserToMessages, err := uc.userConnector.SubscribeUserToMessages(uc, uc.Id)
	if err != nil {
		return err
	}
	termUserToChatMessages, err := uc.userConnector.SubscribeUserToChatMessages(uc, uc.Id)
	if err != nil {
		return err
	}
	uc.term.Add(1)
	go func() {
		uc.term.Wait()
		uc.logger.Debug("connection terminated wait group", uc.Id)
		termUserToChatMessages <- struct{}{}
		termUserToMessages <- struct{}{}
	}()
	go uc.HandleRead()
	uc.HandleWrite()
	return nil
}

func (uc *UserConnection) Send(message types.Message) error {
	uc.sendChan <- message
	return nil
}

func (uc *UserConnection) HandleRead() {
	defer func() {
		uc.logger.Debug("Close")
		uc.conn.Close()
	}()
	uc.conn.SetPongHandler(func(string) error {
		uc.logger.Debug("Pong")
		return nil
	})
	uc.logger.Debug("Starting Loop")
	for {
		uc.logger.Debug("Reading message")
		mt, message, err := uc.conn.ReadMessage()
		uc.logger.Debug("Got message: ", err)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				uc.logger.Warn("error: %v", err)
			}
			break
		}

		if mt != websocket.BinaryMessage {
			uc.logger.Debug("Message is not binary:", mt)
			continue
		}

		// Handle message
		msg, err := types.DecomposeMessage(message)
		if err != nil {
			continue
		}
		messageType := msg.MessageType
		rawMessage := msg.Data

		uc.logger.Debug("Got message: ", messageType)
		switch messageType {
		case types.MessageTypeConnect:
			body := types.ConnectWithNameMessage{}
			err = types.DecodeMessage(&body, rawMessage)
			if err != nil {
				uc.logger.Debug("Error decoding body: ", err)
				break
			}
			uc.logger.Debug("ConnectWithNameMessage: ", body)

			uc.sendChan <- types.Message{MessageType: types.MessageTypeConnected, Data: types.EncodeMessageOrPanic(types.UserInfo{Id: uc.Id, Pub: body.Pub})}
			err = uc.userConnector.ConnectToChat(uc, uc.Id, body.Pub)
			uc.logger.Debug("Connected to chat: ", uc.Id)
			if err != nil {
				uc.logger.Warn("Error connecting to chat: ", err)
				uc.sendChan <- types.Message{MessageType: types.MessageTypeConnectionError}
				break
			}
		case types.MessageTypeMessageToUser:
			body := types.MessageToUser{}
			err = types.DecodeMessage(&body, rawMessage)
			if err != nil {
				uc.logger.Debug("Error decoding body: ", err)
				break
			}
			uc.logger.Debug("MessageToUser: ", body)

			err = uc.messenger.SendMessage(body.UserId, message)
			if err != nil {
				uc.sendChan <- types.Message{MessageType: types.MessageTypeError}
				break
			}
		}
	}
	uc.logger.Debug("Ending Loop")
}

func (uc *UserConnection) HandleWrite() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		uc.logger.Debug("closing write handler for", uc.Id)
		uc.term.Done()
		ticker.Stop()
		uc.userConnector.Disconnect(uc, uc.Id)
		close(uc.sendChan)
		uc.conn.Close()
	}()

	for {
		select {
		case message, ok := <-uc.sendChan:
			uc.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				uc.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := uc.conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				return
			}
			w.Write(types.ComposeMessage(message.MessageType, message.Data))

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			uc.conn.SetWriteDeadline(time.Now().Add(writeWait))
			uc.logger.Debug("Ping:", uc.Id)
			if err := uc.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				uc.logger.Debug("pong err ", uc.Id)
				return
			}
		}
	}
}
