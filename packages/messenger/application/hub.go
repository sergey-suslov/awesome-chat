package application

import (
	"errors"

	"github.com/sergey-suslov/awesome-chat/common"
	"github.com/sergey-suslov/awesome-chat/common/types"
)

type MessageBroker interface {
	SubscribeForClientMessages(id, tag string) (chan types.Message, common.TermChan, error)
}

type ClientConnection struct {
	uc                      *UserConnection
	nickname                string
	fromBroker              <-chan types.Message
	unsubscribeFromMessages common.TermChan
	pub                     string
}

type ChatRoom struct {
	host    *ClientConnection
	invited *ClientConnection
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

func (h *Hub) AddConnection(nickname string, pub string, uc *UserConnection) error {
	_, existsId := h.usersById[uc.Id]
	_, existsTag := h.usersByTag[nickname]
	if existsId || existsTag {
		return errors.New("Connection exists")
	}
	fromBroker, unsubscribe, err := h.mb.SubscribeForClientMessages(uc.Id, nickname)
	if err != nil {
		return errors.New("Could noot subscribe")
	}
	go pipeBrokerMessages(fromBroker, uc.Send)
	cc := &ClientConnection{uc: uc, nickname: nickname, fromBroker: fromBroker, pub: pub, unsubscribeFromMessages: unsubscribe}
	h.usersById[uc.Id] = cc
	h.usersByTag[nickname] = cc
	return nil
}

func (h *Hub) Disconnect(uc *UserConnection) {
	cc, exists := h.usersById[uc.Id]
	if !exists {
		return
	}
	common.SafeSend(cc.unsubscribeFromMessages, struct{}{})
	delete(h.usersById, uc.Id)
	delete(h.usersByTag, cc.nickname)

}
func (h *Hub) CreateRoomWithUserByTag(userTag string, uc *UserConnection) error {
	return nil
}
