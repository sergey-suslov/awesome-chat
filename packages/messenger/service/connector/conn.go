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
	SubscribeToChatMessages(id string, cb func(m types.Message)) (common.TermChan, error)
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
	return &ConnectorService{mb: mb, users: []types.UserInfo{}, uLock: sync.Mutex{}, logger: logger, clientConnById: make(map[string]shared.ClientConnection)}
}

func (cs *ConnectorService) Run() error {
	cs.logger.Debug("Starting ConnectorService")
	term, err := cs.mb.SubscribeToRoomUpdate(func(m types.Message) {
		switch m.MessageType {
		case types.MessageTypeNewUserConnected:
			cs.logger.Debug("connector: new user connected")
			body := types.UserPayload{}
			err := types.DecodeMessage(&body, m.Data)
			if err != nil {
				cs.logger.Debug("Error decoding body: ", err)
				break
			}
			cs.logger.Debugf("Add update user %s", body.User.Id)
			cs.addUpdateUser(body.User)
		case types.MessageTypeUserDisconnected:
			cs.logger.Debug("connector: user disconnected")
			body := types.UserPayload{}
			err := types.DecodeMessage(&body, m.Data)
			if err != nil {
				cs.logger.Debug("Error decoding body: ", err)
				break
			}
			cs.logger.Debugf("Remove user %s", body.User.Id)
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
	cs.logger.Debug("Locked uLock")
	cs.uLock.Lock()
	// defer cs.uLock.Unlock()
	defer func() {
		cs.uLock.Unlock()
		cs.logger.Debug("Unlocked uLock")
	}()
	for i, ui := range cs.users {
		if user.Id == ui.Id {
			cs.users[i] = user
			return
		}
	}
	cs.users = append(cs.users, user)
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

	cs.logger.Debugf("connecting user %s to chat", id)
	err := cs.mb.NotifyOnNewUser(id, pub)
	if err != nil {
		return err
	}

	cs.logger.Debug("sending user infos", cs.users)
	err = cc.Send(types.Message{MessageType: types.MessageTypeUserInfos, Data: types.EncodeMessageOrPanic(types.UserInfosMessage{Users: cs.users})})
	if err != nil {
		return err
	}
	cs.logger.Debug("Before addUpdateUser")
	cs.addUpdateUser(types.UserInfo{Id: id, Pub: pub})
	cs.clientConnById[id] = cc

	cs.logger.Debug("User added: ", id)
	return nil
}

func (cs *ConnectorService) Disconnect(cc shared.ClientConnection, id string) error {
	cs.logger.Debugf("disconnecting user %s from chat", id)
	delete(cs.clientConnById, id)
	cs.removeUser(types.UserInfo{Id: id})
	err := cs.mb.NotifyOnUserDisconnect(id)
	if err != nil {
		return err
	}

	return nil
}

func (cs *ConnectorService) SubscribeUserToMessages(cc shared.ClientConnection, id string) (common.TermChan, error) {
	var term common.TermChan
	cs.logger.Debugf("subscribing user %s to broker messages", id)
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
	if err != nil {
		cs.logger.Warnf("error subscribing user %s to broker messages", id)
		cs.logger.Warn("error:", err)
		return nil, err
	}
	return term, nil
}

func (cs *ConnectorService) SubscribeUserToChatMessages(cc shared.ClientConnection, id string) (common.TermChan, error) {
	var term common.TermChan
	cs.logger.Debugf("subscribing user %s to chat messages", id)
	term, err := cs.mb.SubscribeToChatMessages(id, func(m types.Message) {
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
	if err != nil {
		cs.logger.Warnf("error subscribing user %s to broker messages", id)
		cs.logger.Warn("error:", err)
		return nil, err
	}
	return term, nil
}
