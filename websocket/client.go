package websocket

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/farnese17/chat/config"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	System = iota + 100
	Chat
	Broadcast
	UpdateBlackList
	Ack
)

const (
	Join = iota + 201
	Leave
	Create
	Destroy
	Logout
	Invite
	Apply
)

type Client struct {
	id      uint
	conn    *websocket.Conn
	send    chan any
	closed  bool
	service Service
}

func NewWsClient(s Service, id uint, conn *websocket.Conn) *Client {
	return &Client{
		id:      id,
		conn:    conn,
		send:    make(chan any),
		closed:  false,
		service: s,
	}
}

type CloseSignal struct{}

func (c *Client) Read() {
	defer func() {
		if !c.closed {
			c.closed = true
			c.send <- CloseSignal{}
		}
	}()

	for {
		_, p, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) ||
				strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			c.service.Logger().Error("Failed to read websocket message", zap.Error(err), zap.Uint("client_id", c.id))
			return
		}

		var msg *Message
		if err := json.Unmarshal(p, &msg); err != nil {
			c.service.Logger().Warn("Failed to unmarsal JSON message", zap.Error(err), zap.String("message", string(p)))
			continue
		}

		body := msg.Body
		switch msg.Type {
		case Chat:
			msg, err := c.parseMessage(body)
			if err != nil {
				return
			}
			c.service.Hub().SendToChat(msg)
		case Broadcast:
			msg, err := c.parseMessage(body)
			if err != nil {
				return
			}
			c.service.Hub().SendToBroadcast(msg)
		case Ack:
			msg, err := c.parseAckMessage(body)
			if err != nil {
				return
			}
			c.service.Hub().SendToAck(msg)
		default:
			c.service.Logger().Error("Unknow websocket message type", zap.String("message", string(p)))
			return
		}
	}
}

func (c *Client) Write() {
	defer func() {
		c.service.Hub().Unregister(c)
	}()

	for msg := range c.send {
		if c.closed {
			c.sendCloseMessage()
			c.conn.Close()
			close(c.send)
			if _, ok := msg.(CloseSignal); !ok {
				c.service.Hub().StoreOfflineMessage(msg, c.id)
			}
			for msg := range c.send {
				if _, ok := msg.(CloseSignal); !ok {
					c.service.Hub().StoreOfflineMessage(msg, c.id)
				}
			}
			return
		}
		var packagingMsg struct {
			Type int `json:"type"`
			Body any `json:"body"`
		}
		switch m := msg.(type) {
		case *ChatMsg:
			packagingMsg.Type = m.Type
			packagingMsg.Body = m
		case *AckMsg:
			packagingMsg.Type = m.Type
			packagingMsg.Body = m
		default:
			c.service.Logger().Error("Unknown message type")
			continue
		}

		sent := false
		if err := c.conn.WriteJSON(packagingMsg); err != nil {
			c.service.Logger().Error("Failed to send message", zap.Error(err))
			maxRetries := config.GetConfig().Common().MaxRetries()
			for try := 0; try < maxRetries; try++ {
				logMsg := fmt.Sprintf("Failed to send message,start retrying: %d times", try)
				c.service.Logger().Error(logMsg, zap.Error(err))
				if err := c.conn.WriteJSON(packagingMsg); err != nil {
					delay := config.GetConfig().Common().RetryDelay(try)
					time.Sleep(delay)
					continue
				}
				sent = true
				break
			}
			if !sent {
				c.service.Hub().StoreOfflineMessage(msg, c.id)
			}
		}
	}
}

type Message struct {
	Type int             `json:"type"`
	Body json.RawMessage `json:"body"`
}

type ChatMsg struct {
	ID    string `json:"id"`
	Type  int    `json:"type"`
	From  uint   `json:"from"`
	Body  string `json:"body"`
	Time  int64  `json:"time"`
	To    uint   `json:"to"`
	Extra any    `json:"extra"`
}

type AckMsg struct {
	Type int    `json:"type"`
	ID   string `json:"id"`
	To   uint   `json:"to"`
	Time int64  `json:"time"`
}

func (c *Client) parseMessage(data json.RawMessage) (*ChatMsg, error) {
	var msg *ChatMsg
	err := json.Unmarshal(data, &msg)
	return msg, c.handleJsonError(err, data)
}

func (c *Client) parseAckMessage(data json.RawMessage) (*AckMsg, error) {
	var msg *AckMsg
	err := json.Unmarshal(data, &msg)
	return msg, c.handleJsonError(err, data)
}

func (c *Client) handleJsonError(err error, data json.RawMessage) error {
	if err != nil {
		c.service.Logger().Error("Unknow websocket message type", zap.String("message", string(data)))
		return err
	}
	return nil
}

func (c *Client) sendCloseMessage() {
	message := websocket.FormatCloseMessage(
		websocket.CloseServiceRestart,
		errorsx.ErrServerClosed.Error())
	c.conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(3*time.Second))
}
