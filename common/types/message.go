package types

const (
	MessageTypeConnect = 1 + iota
	MessageTypeNewRoom
)

type Message struct {
	MessageType byte
	Data        []byte
}

type ConnectWithNameMessage struct {
	name string
}
