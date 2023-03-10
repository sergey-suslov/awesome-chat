package messenger

import (
	"go.uber.org/zap"
)

type MessageBroker interface {
	SendMessageToUser(userId string, data []byte) error
}

type MessengerService struct {
	mb     MessageBroker
	logger *zap.SugaredLogger
}

func NewMessengerService(mb MessageBroker, logger *zap.SugaredLogger) *MessengerService {
	return &MessengerService{mb: mb, logger: logger}
}

func (ms *MessengerService) SendMessage(userId string, data []byte) error {
	ms.logger.Debugf("sending message to user with id %s", userId)
	return ms.mb.SendMessageToUser(userId, data)
}
