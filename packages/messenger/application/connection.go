package application

import (
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sergey-suslov/awesome-chat/common/types"
	"go.uber.org/zap"
)

type UserConnector interface {
	AddConnection(nickname string, pub string, uc *UserConnection) error
	Disconnect(uc *UserConnection)
	CreateRoomWithUserByTag(userTag string, uc *UserConnection) error
	SendRoomInvitationAccept(roomId, pub string) error
}

type UserConnection struct {
	conn          *websocket.Conn
	logger        *zap.SugaredLogger
	Send          chan types.Message
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
	return &UserConnection{Id: string(uuid.New().String()), conn: userConnection, logger: logger, Send: send, userConnector: userConnector}
}

func (uc *UserConnection) Run() {
	go uc.HandleRead()
	go uc.HandleWrite()
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

			err = uc.userConnector.AddConnection(body.Name, body.Pub, uc)
			if err != nil {
				uc.Send <- types.Message{MessageType: types.MessageTypeConnectionError}
				break
			}
			uc.Send <- types.Message{MessageType: types.MessageTypeConnectionError}

		case types.MessageTypeNewRoom:
			body := types.NewRoomByUserTagMessage{}
			err = types.DecodeMessage(&body, rawMessage)
			if err != nil {
				uc.logger.Debug("Error decoding body: ", err)
				break
			}

			err = uc.userConnector.CreateRoomWithUserByTag(body.UserTag, uc)
			if err != nil {
				uc.Send <- types.Message{MessageType: types.MessageTypeRoomCreationError}
				break
			}
		case types.MessageTypeNewRoomInviteAccepted:
			body := types.InvitationAcceptedMessage{}
			err = types.DecodeMessage(&body, rawMessage)
			if err != nil {
				uc.logger.Debug("Error decoding body: ", err)
				break
			}
			err = uc.userConnector.SendRoomInvitationAccept(body.RoomId, body.Pub)
			if err != nil {
				uc.logger.Debug("Error sending room invitation accept ")
				break
			}
		}
	}
}

func (uc *UserConnection) HandleWrite() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		uc.userConnector.Disconnect(uc)
		close(uc.Send)
		uc.conn.Close()
	}()

	for {
		select {
		case message, ok := <-uc.Send:
			uc.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				uc.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// TODO: pass message type too
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
