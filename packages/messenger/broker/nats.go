package broker

import (
	"github.com/sergey-suslov/awesome-chat/common"
	"github.com/sergey-suslov/awesome-chat/common/types"
)

type NatsBroker struct {
}

func NewNatsBroker() *NatsBroker {
	return &NatsBroker{}
}

func (nb *NatsBroker) SendMessageByTag(tag string, message types.Message) error {
	return nil
}

func (nb *NatsBroker) SendMessageById(userId string, message types.Message) error {
	return nil
}

func (nb *NatsBroker) SubscribeForRoomInvitationAccept(roomId string) (chan types.InvitationAcceptedMessage, common.TermChan, error) {
	return nil, nil, nil
}

func (nb *NatsBroker) SubscribeForClientMessages(id, tag string) (chan types.Message, common.TermChan, error) {
	return nil, nil, nil
}
