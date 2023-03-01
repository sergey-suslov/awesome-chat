package application

import (
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sergey-suslov/awesome-chat/common/types"
	"github.com/sergey-suslov/awesome-chat/packages/messenger/shared"
	"go.uber.org/zap"
)

type UserConnector interface {
	ConnectToChat(cc shared.ClientConnection, id, pub string) error
	Disconnect(cc shared.ClientConnection, id string)
}

type UserConnection struct {
	conn          *websocket.Conn
	logger        *zap.SugaredLogger
	sendChan      chan types.Message
	userConnector UserConnector
	Id            string
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

func NewUserConnection(userConnection *websocket.Conn, logger *zap.SugaredLogger, send chan types.Message, userConnector UserConnector) *UserConnection {
	return &UserConnection{Id: string(uuid.New().String()), conn: userConnection, logger: logger, sendChan: send, userConnector: userConnector}
}

func (uc *UserConnection) Run() {
	go uc.HandleRead()
	go uc.HandleWrite()
}

func (uc *UserConnection) Send(message types.Message) error {
	return nil
}

func (uc *UserConnection) HandleRead() {
	defer func() {
		uc.conn.Close()
	}()
	uc.conn.SetReadLimit(maxMessageSize)
	uc.conn.SetReadDeadline(time.Now().Add(pongWait))
	uc.conn.SetPongHandler(func(string) error { uc.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		mt, message, err := uc.conn.ReadMessage()
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
		if len(message) < 2 {
			uc.logger.Debug("Message is too short:", len(message))
			continue
		}

		// Handle message
		messageType := message[0]
		rawMessage := message[1:]

		switch messageType {
		case types.MessageTypeConnect:
			body := types.ConnectWithNameMessage{}
			err = types.DecodeMessage(&body, rawMessage)
			if err != nil {
				uc.logger.Debug("Error decoding body: ", err)
				break
			}
			uc.logger.Debug("ConnectWithNameMessage: ", body)

			err = uc.userConnector.ConnectToChat(uc, uc.Id, body.Pub)
			if err != nil {
				uc.sendChan <- types.Message{MessageType: types.MessageTypeConnectionError}
				break
			}
		}
	}
}

func (uc *UserConnection) HandleWrite() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
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
			if err := uc.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
