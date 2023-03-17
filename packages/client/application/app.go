package application

import (
	"bufio"
	"errors"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sergey-suslov/awesome-chat/common/types"
	"github.com/sergey-suslov/awesome-chat/common/util"
	"go.uber.org/zap"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 5 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 100000000

	messageEcriptionLabel = "message"

	connTermintedNormaly uint8 = 1
	connTermintedWithErr uint8 = 2
)

type Application struct {
	logger    *zap.SugaredLogger
	conn      *websocket.Conn
	connTerm  chan uint8
	userInfos []*UserInfoLocal
	id        string
	keyPair   *KeyPair
	connected bool
}

func NewApplication(logger *zap.SugaredLogger, keyPair *KeyPair) *Application {
	return &Application{logger: logger, keyPair: keyPair, connected: false, connTerm: make(chan uint8, 2)}
}

func (app *Application) handleRead() {
	for {
		app.logger.Debug("Reading")
		_, msg, err := app.conn.ReadMessage()
		app.logger.Debug("Got message")
		if err != nil {
			app.connTerm <- connTermintedWithErr
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				app.logger.Warn("error: %v", err)
				return
			}
			return
		}
		m, _ := types.DecomposeMessage(msg)
		app.logger.Debug("Received: ", m)
		switch m.MessageType {
		case types.MessageTypeUserInfos:
			body := types.UserInfosMessage{}
			err = types.DecodeMessage(&body, m.Data)
			app.logger.Debug("user id: ", app.id)
			app.logger.Debug("received infos: ", body)
			userInfos := util.Filter(body.Users, func(u types.UserInfo) bool {
				return u.Id != app.id
			})
			app.userInfos = util.Map(userInfos, func(ui types.UserInfo) *UserInfoLocal {
				return NewUserInfoLocal(ui)
			})
			app.logger.Debug("user infos applied: ", app.userInfos)
		case types.MessageTypeConnected:
			body := types.UserInfo{}
			err = types.DecodeMessage(&body, m.Data)
			app.logger.Debug(err)
			app.logger.Debug(body)
			app.logger.Debug("User connected ", body.Id)
			app.id = body.Id
		case types.MessageTypeNewUserConnected:
			app.logger.Debug("connector: new user connected")
			body := types.UserPayload{}
			err := types.DecodeMessage(&body, m.Data)
			if err != nil {
				app.logger.Debug("Error decoding body: ", err)
				break
			}
			if body.User.Id == app.id {
				break
			}
			app.logger.Debugf("Add update user %s", body.User.Id)
			app.userInfos = append(app.userInfos, NewUserInfoLocal(body.User))
		case types.MessageTypeUserDisconnected:
			app.logger.Debug("connector: user disconnected")
			body := types.UserPayload{}
			err := types.DecodeMessage(&body, m.Data)
			if err != nil {
				app.logger.Debug("Error decoding body: ", err)
				break
			}
			if body.User.Id == app.id {
				break
			}
			app.logger.Debugf("Add update user %s", body.User.Id)
			app.userInfos = util.Filter(app.userInfos, func(u *UserInfoLocal) bool {
				return u.ui.Id != body.User.Id
			})
		case types.MessageTypeMessageToUser:
			body := types.MessageToUser{}
			err = types.DecodeMessage(&body, m.Data)
			app.logger.Debugf("Message got [%s]: %s", body.UserId, string(body.Data))
			plaintext, _ := app.keyPair.DecryptWithPriv(body.Data, []byte(messageEcriptionLabel))
			app.logger.Debugf("Message got decripted [%s]: %s", body.UserId, string(plaintext))
		}
	}
}

func (app *Application) setupConnection(url string) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	conn.SetPingHandler(func(appData string) error {
		// app.logger.Debug("Ping")
		return nil
	})
	return conn, nil
}

func (app *Application) Run(url string) error {
	tries := 2
	for {
		app.logger.Debug("Trying to connect...")
		conn, err := app.setupConnection(url)
		if err != nil {
			app.logger.Fatal("Error connecting to Websocket Server:", err)
		}
		app.conn = conn
		app.connTerm = make(chan uint8, 2)
		go app.handleRead()
		err = app.ConnectWitPub()
		if err != nil {
			app.logger.Fatal("Error registering user:", err)
		}
		err = app.StartLoop()
		app.conn.Close()
		if err == nil {
			return nil
		}
		if err != nil && tries < 0 {
			return err
		}
		app.logger.Debug("Trying to reconnect in 3 seconds...")
		time.Sleep(time.Second * 3)
	}
}

func (app *Application) ConnectWitPub() error {
	app.conn.SetWriteDeadline(time.Now().Add(writeWait))
	pub, err := app.keyPair.PublicKeyToX509()
	if err != nil {
		return err
	}
	message, _ := types.EncodeMessage(types.ConnectWithNameMessage{Name: uuid.NewString(), Pub: pub})
	err = app.conn.WriteMessage(websocket.BinaryMessage, types.ComposeMessage(types.MessageTypeConnect, message))
	if err != nil {
		app.logger.Warn("Error during writing to websocket:", err)
		return err
	}
	return nil
}

func (app *Application) ConstructMessages(content string) []types.MessageToUser {
	messages := make([]types.MessageToUser, len(app.userInfos))
	for i, ui := range app.userInfos {
		encripted, err := app.keyPair.EncryptWithPub([]byte(content), []byte(messageEcriptionLabel), ui.pub)
		if err != nil {
			continue
		}
		messages[i] = types.MessageToUser{UserId: ui.ui.Id, Data: encripted}
	}
	return messages
}

func (app *Application) StartLoop() error {
	interrupt := make(chan os.Signal) // Channel to listen for interrupt signal to terminate gracefully

	signal.Notify(interrupt, os.Interrupt) // Notify the interrupt channel for SIGINT
	for {
		select {
		default:
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if strings.Contains(input, "close") {
				go func() {
					interrupt <- os.Kill
				}()
				return nil
			}
			if err != nil {
				app.logger.Error("An error occured while reading input. Please try again", err)
				return nil
			}

			// remove the delimeter from the string
			input = strings.TrimSuffix(input, "\n")

			// app.ConnectWitPub()
			messages := app.ConstructMessages(input)
			for _, mtu := range messages {
				app.conn.SetWriteDeadline(time.Now().Add(writeWait))
				app.logger.Debug("Writing message", mtu)
				msg := types.EncodeMessageOrPanic(mtu)

				app.conn.SetWriteDeadline(time.Now().Add(writeWait))
				w, err := app.conn.NextWriter(websocket.BinaryMessage)
				if err != nil {
					return nil
				}
				w.Write(types.ComposeMessage(types.MessageTypeMessageToUser, msg))

				if err := w.Close(); err != nil {
					return nil
				}
				// err = app.conn.WriteMessage(websocket.BinaryMessage, types.ComposeMessage(types.MessageTypeMessageToUser, msg))
				// app.logger.Debug("Message written:", msg, err)
				// if err != nil {
				// 	app.logger.Error("Error during writing to websocket:", err)
				// 	continue
				// }
			}

		case termCode := <-app.connTerm:
			if termCode == connTermintedWithErr {
				app.logger.Debug("Terminated with error")
				return errors.New("Terminated with error")
			}
			app.logger.Debug("Terminated normaly")
			return nil
		case <-interrupt:
			// We received a SIGINT (Ctrl + C). Terminate gracefully...
			app.logger.Debug("Received SIGINT interrupt signal. Closing all pending connections")

			// Close our websocket connection
			err := app.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				app.logger.Debug("Error during closing websocket:", err)
				return err
			}

			return nil
		}
	}
}
