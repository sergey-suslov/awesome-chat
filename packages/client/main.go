package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/gorilla/websocket"
	"github.com/sergey-suslov/awesome-chat/common"
	"github.com/sergey-suslov/awesome-chat/common/types"
	"github.com/sergey-suslov/awesome-chat/packages/client/application"
	"go.uber.org/zap"
)

var done chan interface{}
var interrupt chan os.Signal

func receiveHandler(connection *websocket.Conn) {
	defer close(done)
	for {
		_, msg, err := connection.ReadMessage()
		if err != nil {
			log.Println("Error in receive:", err)
			return
		}
		m, _ := types.DecomposeMessage(msg)
		log.Printf("Received: %d\n", m.MessageType)
		switch m.MessageType {
		case types.MessageTypeUserInfos:
			body := types.UserInfosMessage{}
			err = types.DecodeMessage(&body, m.Data)
			log.Println(err)
			log.Println(body)
		case types.MessageTypeConnected:
			body := types.UserInfo{}
			err = types.DecodeMessage(&body, m.Data)
			log.Println(err)
			log.Println(body)
		}
	}
}

func main() {
	fs := flag.NewFlagSet("chat-client", flag.ExitOnError)
	var (
		env  = fs.String("env", "local", "Environment (local, development, test, production)")
		port = fs.Int("port", 50050, "Port")
		help = fs.Bool("h", false, "Show help")
	)
	fs.Usage = usageFor(fs, os.Args[0]+" [flags] <a> <b>")
	_ = fs.Parse(os.Args[1:])
	if *help {
		fs.Usage()
		os.Exit(1)
	}

	socketUrl := fmt.Sprintf("ws://localhost:%d/ws", *port)
	var zapLogger *zap.Logger
	if zapLogger, _ = zap.NewDevelopment(); *env == "production" {
		zapLogger, _ = zap.NewProduction()
	}
	logger := zapLogger.Sugar()
	common.SetLogger(logger)
	keyPair, err := application.GenerateKeyPair()
	if err != nil {
		logger.Panic(err)
	}
	app := application.NewApplication(logger, keyPair)
	err = app.Run(socketUrl)
	if err != nil {
		logger.Panic(err)
	}
}

func usageFor(fs *flag.FlagSet, short string) func() {
	return func() {
		fmt.Fprintf(os.Stderr, "USAGE\n")
		fmt.Fprintf(os.Stderr, "  %s\n", short)
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		w := tabwriter.NewWriter(os.Stderr, 0, 2, 2, ' ', 0)
		fs.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(w, "\t-%s %s\t%s\n", f.Name, f.DefValue, f.Usage)
		})
		_ = w.Flush()
		fmt.Fprintf(os.Stderr, "\n")
	}
}
