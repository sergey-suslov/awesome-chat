package broker

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/sergey-suslov/awesome-chat/common"
	"github.com/sergey-suslov/awesome-chat/common/types"
	"github.com/sourcegraph/conc"
	"go.uber.org/zap"
)

type NatsBroker struct {
	nc             *nats.Conn
	js             nats.JetStreamContext
	logger         *zap.SugaredLogger
	usersObjStore  nats.ObjectStore
	usersPubs      map[string][]byte
	roomUpdateSubs map[string]func(m types.Message)
}

func NewNatsBroker(nc *nats.Conn, logger *zap.SugaredLogger) (*NatsBroker, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}
	objStore, err := js.ObjectStore("users")
	if err == nats.ErrStreamNotFound {
		err = nil
		objStore, err = js.CreateObjectStore(&nats.ObjectStoreConfig{
			Bucket:  "users",
			Storage: nats.FileStorage,
			TTL:     time.Duration(time.Hour * 24),
		})
		if err != nil {
			return nil, err
		}
	}

	userPubs, err := fetchUserPubsFromNats(objStore)
	if err != nil {
		return nil, err
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "messages",
		Subjects: []string{"messages.>"},
	})
	if err != nats.ErrStreamNameAlreadyInUse && err != nil {
		return nil, err
	}
	return &NatsBroker{nc: nc, js: js, logger: logger, usersObjStore: objStore, usersPubs: userPubs, roomUpdateSubs: make(map[string]func(m types.Message))}, nil
}

func (nb *NatsBroker) NotifyOnNewUser(id string, pub []byte) error {
	_, err := nb.usersObjStore.PutBytes(id, pub)
	return err
}

func (nb *NatsBroker) NotifyOnUserDisconnect(id string) error {
	err := nb.usersObjStore.Delete(id)
	return err
}

func (nb *NatsBroker) Run() (common.TermChan, error) {
	watcher, err := nb.usersObjStore.Watch()
	if err != nil {
		return nil, err
	}
	term := make(common.TermChan, 1)
	go func() {
		u := watcher.Updates()
		for {
			select {
			case <-term:
				watcher.Stop()
				break
			case update := <-u:
				if update == nil {
					continue
				}
				if update.Deleted {
					delete(nb.usersPubs, update.Name)
					nb.notifySubsOnRoomUpdate(update.Name, true)
					continue
				}
				pub, err := nb.usersObjStore.GetBytes(update.Name)
				if err != nil {
					return
				}
				nb.usersPubs[update.Name] = pub
				nb.notifySubsOnRoomUpdate(update.Name, false)
			}
		}
	}()
	return term, nil
}

func (nb *NatsBroker) notifySubsOnRoomUpdate(id string, deleted bool) error {
	mtype := types.MessageTypeUserDisconnected
	if !deleted {
		mtype = types.MessageTypeNewUserConnected
	}
	pub := nb.usersPubs[id]
	msg := types.Message{MessageType: mtype, Data: types.EncodeMessageOrPanic(types.UserPayload{User: types.UserInfo{Id: id, Pub: pub}})}
	for _, cb := range nb.roomUpdateSubs {
		cb(msg)
	}
	return nil
}

func (nb *NatsBroker) SubscribeToRoomUpdate(cb func(m types.Message)) (common.TermChan, error) {
	term := make(common.TermChan, 1)
	id := uuid.NewString()
	nb.roomUpdateSubs[id] = cb
	go func() {
		<-term
		delete(nb.roomUpdateSubs, id)
	}()
	return term, nil
}

func (nb *NatsBroker) GetUsersInfos() []types.UserInfo {
	ui := make([]types.UserInfo, len(nb.usersPubs))
	for k, v := range nb.usersPubs {
		ui = append(ui, types.UserInfo{Id: k, Pub: v})
	}
	return ui
}

func (nb *NatsBroker) SubscribeToUserMessages(id string, cb func(m types.Message)) (common.TermChan, error) {
	return nb.subscribe(fmt.Sprintf("user.%s.*", id), cb)
}

func (nb *NatsBroker) SubscribeToChatMessages(id string, cb func(m types.Message)) (common.TermChan, error) {
	return nb.subscribeStream(fmt.Sprintf("messages.%s.e", id), cb)
}

func (nb *NatsBroker) SendMessageToUser(userId string, data []byte) error {
	_, err := nb.js.Publish(fmt.Sprintf("messages.%s.e", userId), data)
	return err
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

		nb.logger.Debug("topic:", topic)
		nb.logger.Debug("message:", m)
		cb(m)
		msg.Ack()
	})
	if err != nil {
		return nil, err
	}
	go func() {
		<-term
		nb.logger.Debugf("subscription %s terminated", topic)
		s.Unsubscribe()
	}()
	return term, nil
}

func (nb *NatsBroker) subscribeStream(topic string, cb func(m types.Message)) (common.TermChan, error) {
	term := make(common.TermChan)
	s, err := nb.js.Subscribe(topic, func(msg *nats.Msg) {
		if len(msg.Data) < 1 {
			nb.logger.Debug("Message is too short:", len(msg.Data))
			return
		}
		m, err := types.DecomposeMessage(msg.Data)
		if err != nil {
			return
		}

		cb(m)
		msg.Ack()
	})
	if err != nil {
		return nil, err
	}
	go func() {
		<-term
		nb.logger.Debugf("subscription stream %s terminated", topic)
		s.Unsubscribe()
	}()
	return term, nil
}

func fetchUserPubsFromNats(objStore nats.ObjectStore) (map[string][]byte, error) {
	userPubsObjInfos, err := objStore.List()
	if err == nats.ErrNoObjectsFound {
		return map[string][]byte{}, nil
	}
	if err != nil {
		return nil, err
	}
	var wg conc.WaitGroup
	var userObjStoreInitErr error
	userPubs := make(map[string][]byte)
	for _, userId := range userPubsObjInfos {
		wg.Go(func() {
			pub, err := objStore.GetBytes(userId.ObjectMeta.Name)
			if err != nil {
				userObjStoreInitErr = err
				return
			}
			userPubs[userId.ObjectMeta.Name] = pub
		})
	}
	wg.Wait()
	if userObjStoreInitErr != nil {
		return nil, err
	}
	return userPubs, nil
}
