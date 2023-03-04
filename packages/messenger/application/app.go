package application

import (
	"fmt"
	"net/http"

	"github.com/nats-io/nats.go"
	"github.com/sergey-suslov/awesome-chat/common/types"
	"github.com/sergey-suslov/awesome-chat/packages/messenger/broker"
	"github.com/sergey-suslov/awesome-chat/packages/messenger/service/connector"
	"github.com/sergey-suslov/awesome-chat/packages/messenger/service/messenger"
	"go.uber.org/zap"
)

type Application struct {
	config Config
	logger *zap.SugaredLogger
}

func NewApplication(config Config, logger *zap.SugaredLogger) Application {
	return Application{config: config, logger: logger}
}

func (app *Application) Start() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		app.logger.Fatal("Error connecting to NATS: ", err)
	}
	natsBroker, err := broker.NewNatsBroker(nc, app.logger)
	if err != nil {
		app.logger.Fatal("Error creating NatsBroker: ", err)
	}
	connector := connector.NewConnectorService(natsBroker, app.logger)
	err = connector.Run()
	if err != nil {
		app.logger.Fatal("Error running ConnectorService: ", err)
	}
	messenger := messenger.NewMessengerService(natsBroker, app.logger)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			app.logger.Warn("Error during connection upgrade: %s", zap.Error(err))
			return
		}
		err = NewUserConnection(conn, app.logger, make(chan types.Message), connector, messenger).Run()
		if err != nil {
			app.logger.Warn("Error running connection", err)
		}
	})
	err = http.ListenAndServe(fmt.Sprintf(":%d", app.config.Port), nil)
	if err != nil {
		app.logger.Fatal("ListenAndServe: ", err)
	}

}
