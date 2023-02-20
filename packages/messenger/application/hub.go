package application

import (
	"errors"

	"github.com/sergey-suslov/awesome-chat/common/types"
)

type MessageBroker interface {
	SendConnected(id string)
	SubscribeForClientMessages(id string) chan types.Message
	UnsubscribeForClientMessages(id string)
}

type ClientConnection struct {
	uc         *UserConnection
	nickname   string
	fromBroker <-chan types.Message
}

type Hub struct {
	usersById  map[string]*ClientConnection
	usersByTag map[string]*ClientConnection
	mb         MessageBroker
}

func NewHub(mb MessageBroker) *Hub {
	return &Hub{usersById: make(map[string]*ClientConnection), usersByTag: make(map[string]*ClientConnection), mb: mb}
}

func pipeBrokerMessages(from, to chan types.Message) {
	for msg := range from {
		to <- msg
	}
}

func (h *Hub) AddConnection(nickname string, uc *UserConnection) error {
	_, existsId := h.usersById[uc.Id]
	_, existsTag := h.usersByTag[nickname]
	if existsId || existsTag {
		return errors.New("Connection exists")
	}
	fromBroker := h.mb.SubscribeForClientMessages(uc.Id)
	go pipeBrokerMessages(fromBroker, uc.Send)
	cc := &ClientConnection{uc: uc, nickname: nickname, fromBroker: fromBroker}
	h.usersById[uc.Id] = cc
	h.usersByTag[nickname] = cc
	return nil
}

func (h *Hub) Disconnect(uc *UserConnection) {
	h.mb.UnsubscribeForClientMessages(uc.Id)
	cc, exists := h.usersById[uc.Id]
	if !exists {
		return
	}
	delete(h.usersById, uc.Id)
	delete(h.usersByTag, cc.nickname)
}
