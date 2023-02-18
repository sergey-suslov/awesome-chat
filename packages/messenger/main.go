package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/sergey-suslov/awesome-chat/common"
	"github.com/sergey-suslov/awesome-chat/packages/messenger/application"
	"go.uber.org/zap"
)

func main() {
	fs := flag.NewFlagSet("bookingcli", flag.ExitOnError)
	var (
		port = fs.Int("port", 50050, "Port")
		env  = fs.String("env", "local", "Environment (local, development, test, production)")
		help = fs.Bool("h", false, "Show help")
	)
	fs.Usage = usageFor(fs, os.Args[0]+" [flags] <a> <b>")
	_ = fs.Parse(os.Args[1:])
	if *help {
		fs.Usage()
		os.Exit(1)
	}

	var zapLogger *zap.Logger
	if zapLogger, _ = zap.NewDevelopment(); *env == "production" {
		zapLogger, _ = zap.NewProduction()
	}
	logger := zapLogger.Sugar()
	common.SetLogger(logger)

	// Init dependencies
	logger.Info("Running on env: %s", *env)
	application := application.NewApplication(application.Config{Port: *port}, logger)
	application.Start()
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
