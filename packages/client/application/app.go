package application

import (
	"bufio"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sergey-suslov/awesome-chat/common/types"
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
)

type Application struct {
	logger     *zap.SugaredLogger
	conn       *websocket.Conn
	userInfos  []types.UserInfo
	id         string
	privateKey string
}

func NewApplication(logger *zap.SugaredLogger) *Application {
	return &Application{logger: logger}
}

func (app *Application) handleRead() {
	for {
		app.logger.Debug("Reading")
		_, msg, err := app.conn.ReadMessage()
		app.logger.Debug("Got message")
		if err != nil {
			app.logger.Warn("error: %v", err)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				app.logger.Warn("error: %v", err)
			}
			return
		}
		m, _ := types.DecomposeMessage(msg)
		app.logger.Debug("Received: ", m)
		switch m.MessageType {
		case types.MessageTypeUserInfos:
			body := types.UserInfosMessage{}
			err = types.DecodeMessage(&body, m.Data)
			app.logger.Debug(err)
			app.logger.Debug(body)
			app.userInfos = body.Users
			app.logger.Debug("user infos", app.userInfos)
		case types.MessageTypeConnected:
			body := types.UserInfo{}
			err = types.DecodeMessage(&body, m.Data)
			app.logger.Debug(err)
			app.logger.Debug(body)
			app.id = body.Id
		case types.MessageTypeMessageToUser:
			body := types.MessageToUser{}
			err = types.DecodeMessage(&body, m.Data)
			app.logger.Debugf("Message got [%s]: %s", body.UserId, string(body.Data))
		}
	}
}

func (app *Application) Run(url string) error {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		app.logger.Fatal("Error connecting to Websocket Server:", err)
	}
	app.conn = conn
	conn.SetPingHandler(func(appData string) error {
		app.logger.Debug("Ping")
		return nil
	})
	defer conn.Close()
	go app.handleRead()
	err = app.ConnectWitPub()
	if err != nil {
		app.logger.Fatal("Error registering user:", err)
	}
	return app.StartLoop()
}

func (app *Application) ConnectWitPub() error {
	app.conn.SetWriteDeadline(time.Now().Add(writeWait))
	message, _ := types.EncodeMessage(types.ConnectWithNameMessage{Name: uuid.NewString(), Pub: "pub-key"})
	err := app.conn.WriteMessage(websocket.BinaryMessage, types.ComposeMessage(types.MessageTypeConnect, message))
	if err != nil {
		app.logger.Warn("Error during writing to websocket:", err)
		return err
	}
	return nil
}

func (app *Application) ConstructMessages(content string) []types.MessageToUser {
	messages := make([]types.MessageToUser, len(app.userInfos))
	for i, ui := range app.userInfos {
		messages[i] = types.MessageToUser{UserId: ui.Id, Data: []byte(content)}
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
