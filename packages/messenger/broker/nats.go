package broker

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/sergey-suslov/awesome-chat/common"
	"github.com/sergey-suslov/awesome-chat/common/types"
)

type NatsBroker struct {
	nc *nats.Conn
}

func NewNatsBroker(nc *nats.Conn) *NatsBroker {
	return &NatsBroker{nc: nc}
}

func (nb *NatsBroker) SendMessageByTag(tag string, message types.Message) error {
	return nb.nc.Publish(fmt.Sprintf("user:%s:%d", tag, int8(message.MessageType)), types.EncodeMessageOrPanic(message))

}

func (nb *NatsBroker) SendMessageById(userId string, message types.Message) error {
	return nb.nc.Publish(fmt.Sprintf("user:%s:%d", userId, int8(message.MessageType)), types.EncodeMessageOrPanic(message))
}

func (nb *NatsBroker) SubscribeForRoomInvitationAccept(roomId string) (chan types.InvitationAcceptedMessage, common.TermChan, error) {
	c := make(chan types.InvitationAcceptedMessage)
	t := make(common.TermChan)

	return c, t, nil
}

func (nb *NatsBroker) SubscribeForClientMessages(id, tag string) (chan types.Message, common.TermChan, error) {
	return nil, nil, nil
}
