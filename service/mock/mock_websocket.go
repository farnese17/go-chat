package mock

import (
	ws "github.com/farnese17/chat/websocket"
)

var Message chan *ws.ChatMsg
var HandleBlock chan *ws.HandleBlockMsg

type MockHub struct {
}

// CacheMessage implements websocket.HubInterface.
func (m *MockHub) CacheMessage(message any, id uint) {
	panic("unimplemented")
}

// Register implements websocket.HubInterface.
func (m *MockHub) Register(c *ws.Client) {
	panic("unimplemented")
}

// Unregister implements websocket.HubInterface.
func (m *MockHub) Unregister(c *ws.Client) {
	panic("unimplemented")
}

// Kick implements websocket.HubInterface.
func (m *MockHub) Kick(id uint) {
	panic("unimplemented")
}

// IsClosed implements websocket.HubInterface.
func (m *MockHub) IsClosed() bool {
	panic("unimplemented")
}

// Count implements websocket.HubInterface.
func (m *MockHub) Count() int {
	panic("unimplemented")
}

func NewMockHub() ws.HubInterface {
	return &MockHub{}
}

func (m *MockHub) Run() {
	Message = make(chan *ws.ChatMsg, 2)
	HandleBlock = make(chan *ws.HandleBlockMsg, 1)
}
func (m *MockHub) Stop() {
	close(Message)
	close(HandleBlock)
}
func (m *MockHub) SendToRegister(client *ws.Client)   {}
func (m *MockHub) SendToUnregister(client *ws.Client) {}
func (m *MockHub) SendToChat(message *ws.ChatMsg) {
	Message <- message
}
func (m *MockHub) SendToBroadcast(message *ws.ChatMsg) {
	Message <- message
}

func (m *MockHub) SendToApply(message *ws.ChatMsg) {
	Message <- message
}
func (m *MockHub) SendDeleteGroupNotify(message *ws.ChatMsg) {
	Message <- message
}

// SendToHandleBlockedMessage implements websocket.HubInterface.
func (m *MockHub) SendUpdateBlockedListNotify(message *ws.HandleBlockMsg) {
	HandleBlock <- message
}
