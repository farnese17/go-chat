package mock

import (
	ws "github.com/farnese17/chat/websocket"
)

var Message chan *ws.ChatMsg
var HandleBlock chan *ws.ChatMsg
var Confirm chan any

type MockHub struct {
}

// SendToAck implements websocket.HubInterface.
func (m *MockHub) SendToAck(message *ws.AckMsg) {
}

// CacheMessage implements websocket.HubInterface.
func (m *MockHub) StoreOfflineMessage(message any, id uint) {
}

// Register implements websocket.HubInterface.
func (m *MockHub) Register(c *ws.Client) {
}

// Unregister implements websocket.HubInterface.
func (m *MockHub) Unregister(c *ws.Client) {
}

// Kick implements websocket.HubInterface.
func (m *MockHub) Kick(id uint) {
	Confirm <- id
}

// IsClosed implements websocket.HubInterface.
func (m *MockHub) IsClosed() bool {
	return false
}

// Count implements websocket.HubInterface.
func (m *MockHub) Count() int {
	return 0
}

func NewMockHub() ws.HubInterface {
	return &MockHub{}
}

func (m *MockHub) Run() {
	Message = make(chan *ws.ChatMsg, 2)
	HandleBlock = make(chan *ws.ChatMsg, 1)
	Confirm = make(chan any)
}
func (m *MockHub) Stop() {
	close(Message)
	close(HandleBlock)
	close(Confirm)
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
func (m *MockHub) SendUpdateBlockedListNotify(message *ws.ChatMsg) {
	HandleBlock <- message
}
