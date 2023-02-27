package application

import (
	"fmt"
	"net/http"

	"github.com/sergey-suslov/awesome-chat/common/types"
	"github.com/sergey-suslov/awesome-chat/packages/messenger/broker"
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
	lb := broker.NewNatsBroker()
	hub := NewHub(lb)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			app.logger.Warn("Error during connection upgrade: %s", zap.Error(err))
			return
		}
		NewUserConnection(conn, app.logger, make(chan types.Message), hub).Run()
	})
	err := http.ListenAndServe(fmt.Sprintf(":%d", app.config.Port), nil)
	if err != nil {
		app.logger.Fatal("ListenAndServe: ", err)
	}

}
