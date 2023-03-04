package connector

import (
	"sync"

	"github.com/sergey-suslov/awesome-chat/common"
	"github.com/sergey-suslov/awesome-chat/common/types"
	"github.com/sergey-suslov/awesome-chat/common/util"
	"github.com/sergey-suslov/awesome-chat/packages/messenger/shared"
	"go.uber.org/zap"
)

type MessageBroker interface {
	NotifyOnNewUser(id, pub string) error
	NotifyOnUserDisconnect(id string) error
	SubscribeToRoomUpdate(cb func(m types.Message)) (common.TermChan, error)
	SubscribeToUserMessages(id string, cb func(m types.Message)) (common.TermChan, error)
}

type ConnectorService struct {
	mb             MessageBroker
	uLock          sync.Mutex
	users          []types.UserInfo
	clientConnById map[string]shared.ClientConnection
	termRoomUpdate common.TermChan
	logger         *zap.SugaredLogger
}

func NewConnectorService(mb MessageBroker, logger *zap.SugaredLogger) *ConnectorService {
	return &ConnectorService{mb: mb, users: make([]types.UserInfo, 1), uLock: sync.Mutex{}, logger: logger, clientConnById: make(map[string]shared.ClientConnection)}
}

func (cs *ConnectorService) Run() error {
	term, err := cs.mb.SubscribeToRoomUpdate(func(m types.Message) {
		switch m.MessageType {
		case types.MessageTypeNewUserConnected:
			body := types.UserPayload{}
			err := types.DecodeMessage(&body, m.Data)
			if err != nil {
				cs.logger.Debug("Error decoding body: ", err)
				break
			}
			cs.addUpdateUser(body.User)
		case types.MessageTypeUserDisconnected:
			body := types.UserPayload{}
			err := types.DecodeMessage(&body, m.Data)
			if err != nil {
				cs.logger.Debug("Error decoding body: ", err)
				break
			}
			cs.removeUser(body.User)
		}
	})
	if err != nil {
		return err
	}
	cs.termRoomUpdate = term
	return nil
}

func (cs *ConnectorService) Stop(cc shared.ClientConnection, id, pub string) error {
	cs.termRoomUpdate <- struct{}{}
	return nil
}

func (cs *ConnectorService) addUpdateUser(user types.UserInfo) {
	cs.uLock.Lock()
	for i, ui := range cs.users {
		if user.Id == ui.Id {
			cs.users[i] = user
			return
		}
	}
	cs.users = append(cs.users, user)
	cs.uLock.Unlock()
}

func (cs *ConnectorService) removeUser(user types.UserInfo) {
	cs.uLock.Lock()
	cs.users = util.Filter(cs.users, func(u types.UserInfo) bool {
		return u.Id != user.Id
	})
	cs.uLock.Unlock()
}

func (cs *ConnectorService) ConnectToChat(cc shared.ClientConnection, id, pub string) error {
	_, exists := cs.clientConnById[id]
	if exists {
		delete(cs.clientConnById, id)
	}

	err := cs.mb.NotifyOnNewUser(id, pub)
	if err != nil {
		return err
	}

	err = cc.Send(types.Message{MessageType: types.MessageTypeUserInfos, Data: types.EncodeMessageOrPanic(types.UserInfosMessage{Users: cs.users})})
	if err != nil {
		return err
	}
	cs.addUpdateUser(types.UserInfo{Id: id, Pub: pub})
	cs.clientConnById[id] = cc

	return nil
}

func (cs *ConnectorService) Disconnect(cc shared.ClientConnection, id string) error {
	delete(cs.clientConnById, id)
	cs.removeUser(types.UserInfo{Id: id})
	err := cs.mb.NotifyOnUserDisconnect(id)
	if err != nil {
		return err
	}

	return nil
}

func (cs *ConnectorService) SubscribeUserToMessages(cc shared.ClientConnection, id string) error {
	var term common.TermChan
	term, err := cs.mb.SubscribeToUserMessages(id, func(m types.Message) {
		cc, existing := cs.clientConnById[id]
		if !existing {
			term <- struct{}{}
			return
		}
		err := cc.Send(m)
		if err != nil {
			term <- struct{}{}
			return
		}
	})
	return err
}
