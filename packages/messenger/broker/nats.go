package broker

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/sergey-suslov/awesome-chat/common"
	"github.com/sergey-suslov/awesome-chat/common/types"
	"go.uber.org/zap"
)

type NatsBroker struct {
	nc     *nats.Conn
	logger *zap.SugaredLogger
}

func NewNatsBroker(nc *nats.Conn, logger *zap.SugaredLogger) *NatsBroker {
	return &NatsBroker{nc: nc, logger: logger}
}

func (nb *NatsBroker) NotifyOnNewUser(id, pub string) error {
	return nb.nc.Publish(
		fmt.Sprintf("room.g.connect"),
		types.ComposeMessage(types.MessageTypeNewUserConnected, types.EncodeMessageOrPanic(types.UserPayload{User: types.UserInfo{Id: id, Pub: pub}})),
	)
}

func (nb *NatsBroker) NotifyOnUserDisconnect(id string) error {
	return nb.nc.Publish(
		fmt.Sprintf("room.g.disconnect"),
		types.ComposeMessage(types.MessageTypeUserDisconnected, types.EncodeMessageOrPanic(types.UserPayload{User: types.UserInfo{Id: id}})),
	)
}

func (nb *NatsBroker) SubscribeToRoomUpdate(cb func(m types.Message)) (common.TermChan, error) {
	return nb.subscribe("room.g.connect", cb)
}

func (nb *NatsBroker) SubscribeToUserMessages(id string, cb func(m types.Message)) (common.TermChan, error) {
	return nb.subscribe(fmt.Sprintf("user.%s.*", id), cb)
}

func (nb *NatsBroker) subscribe(topic string, cb func(m types.Message)) (common.TermChan, error) {
	term := make(common.TermChan)
	s, err := nb.nc.Subscribe(topic, func(msg *nats.Msg) {
		if len(msg.Data) < 1 {
			nb.logger.Debug("Message is too short:", len(msg.Data))
			return
		}
		m, err := types.DecomposeMessage(msg.Data)
		if err != nil {
			return
		}

		cb(m)
	})
	if err != nil {
		return nil, err
	}
	go func() {
		<-term
		s.Unsubscribe()
	}()
	return term, nil
}
