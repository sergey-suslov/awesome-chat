package shared

import (
	"github.com/sergey-suslov/awesome-chat/common/types"
)

type ClientConnection interface {
	Send(message types.Message) error
}
