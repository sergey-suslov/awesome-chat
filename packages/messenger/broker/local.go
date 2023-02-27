package broker

import (
	"errors"

	"github.com/sergey-suslov/awesome-chat/common/types"
)

type LocalBroker struct {
	subsById  map[string]*chan types.Message
	subsByTag map[string]*chan types.Message
}

func NewLocalBroker() *LocalBroker {
	return &LocalBroker{subsById: make(map[string]*chan types.Message), subsByTag: make(map[string]*chan types.Message)}
}

func (lb *LocalBroker) SendMessageById(id string, message types.Message) error {
	sub, exists := lb.subsById[id]
	if !exists {
		return errors.New("Error sending message")
	}
	*sub <- message
	return nil
}

func (lb *LocalBroker) SendMessageByTag(tag string, message types.Message) error {
	sub, exists := lb.subsById[tag]
	if !exists {
		return errors.New("Error sending message")
	}
	*sub <- message
	return nil
}

func (lb *LocalBroker) SubscribeForClientMessages(id string) chan types.Message {
	existing, exists := lb.subsById[id]
	if exists {
		return *existing
	}

	clientC := make(chan types.Message, 1)
	lb.subsById[id] = &clientC
	return clientC
}

func (lb *LocalBroker) UnsubscribeForClientMessages(id string) {
	sub, exists := lb.subsById[id]
	if exists {
		delete(lb.subsById, id)
		close(*sub)
	}
}
