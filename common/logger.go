package common

import "go.uber.org/zap"

var (
	Logger *zap.SugaredLogger
)

func SetLogger(l *zap.SugaredLogger) {
	Logger = l
}

func GetLogger() *zap.SugaredLogger {
	return Logger
}

