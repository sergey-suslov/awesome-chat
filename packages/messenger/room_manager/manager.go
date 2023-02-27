package roommanager

import (
	"errors"

	"github.com/google/uuid"
	"github.com/sergey-suslov/awesome-chat/common"
	"github.com/sergey-suslov/awesome-chat/common/types"
)

type MessageBroker interface {
	SendMessageByTag(tag string, message types.Message) error
	SendMessageById(userId string, message types.Message) error
	SubscribeForRoomInvitationAccept(roomId string) (chan types.InvitationAcceptedMessage, common.TermChan, error)
}

type Room struct {
	Id                    string
	userId                string
	counterpartTag        string
	counterpartPub        string
	ready                 bool
	unsubscribeInvitation common.TermChan
}

type RoomManager struct {
	mb            MessageBroker
	roomsById     map[string]*Room
	roomsByUserId map[string]*Room
}

func NewRoomManager(mb MessageBroker) *RoomManager {
	return &RoomManager{mb: mb, roomsById: map[string]*Room{}, roomsByUserId: map[string]*Room{}}
}

func (rm *RoomManager) Disconnect(userId string) {
	room, exists := rm.roomsByUserId[userId]
	if !exists {
		return
	}
	room.unsubscribeInvitation <- struct{}{}
	delete(rm.roomsById, room.Id)
	delete(rm.roomsByUserId, userId)
}

func (rm *RoomManager) CreateRoom(counterpartTag string, userId string, pub string) error {
	_, exists := rm.roomsByUserId[userId]
	if exists {
		return errors.New("You already in a room")
	}

	room := &Room{Id: uuid.NewString(), userId: userId, counterpartTag: counterpartTag, ready: false}
	rm.roomsById[room.Id] = room
	rm.roomsByUserId[userId] = room

  err := rm.mb.SendMessageByTag(counterpartTag, types.Message{MessageType: types.MessageTypeNewRoomInvite, Data: types.EncodeMessageOrPanic(types.InviteToRoomMessage{RoomId: room.Id, Pub: pub})})
	if err != nil {
		return err
	}

	invitationAcceptedChan, term, err := rm.mb.SubscribeForRoomInvitationAccept(room.Id)
	room.unsubscribeInvitation = term
	if err != nil {
		return err
	}
	go rm.handleInvitation(invitationAcceptedChan, room)
	return nil
}

func (rm *RoomManager) handleInvitation(ic chan types.InvitationAcceptedMessage, room *Room) {
	for m := range ic {
		room.counterpartPub = m.Pub
		room.ready = true
		rm.mb.SendMessageById(room.userId, types.Message{MessageType: types.MessageTypeNewRoomInviteAccepted, Data: types.EncodeMessageOrPanic(m)})
	}
}
