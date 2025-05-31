package websocket

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/farnese17/chat/config"
	repo "github.com/farnese17/chat/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service interface {
	Logger() *zap.Logger
	Config() config.Config
	Cache() repo.Cache
	Hub() HubInterface
	SetHub(hub HubInterface)
}

type HubInterface interface {
	Run()
	Stop()
	Register(c *Client)
	Unregister(c *Client)
	SendToChat(message *ChatMsg)
	SendToBroadcast(message *ChatMsg)
	SendToAck(message *AckMsg)
	SendToApply(message *ChatMsg)
	SendUpdateBlockedListNotify(message *ChatMsg)
	StoreOfflineMessage(message any, id uint)
	Count() int
	IsClosed() bool
	Kick(id uint)
	Uptime() time.Duration
}

func NewHubInterface(service Service) HubInterface {
	return NewHub(service)
}

type MessageContext struct {
	Message any
	To      []uint
	Cache   bool
	Pending bool
	Sent    bool
	Extra   map[string]any
}

type Hub struct {
	clients     map[uint]*Client
	register    chan *Client
	unregister  chan *Client
	chat        chan *ChatMsg
	broadcast   chan *ChatMsg
	done        chan struct{}
	closed      atomic.Bool
	mu          sync.RWMutex
	service     Service
	middlewares []MessageMiddleware
	runningAt   time.Time
}

func NewHub(service Service) *Hub {
	hub := &Hub{
		clients:     make(map[uint]*Client),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		chat:        make(chan *ChatMsg),
		broadcast:   make(chan *ChatMsg),
		done:        make(chan struct{}),
		closed:      atomic.Bool{},
		mu:          sync.RWMutex{},
		service:     service,
		middlewares: []MessageMiddleware{},
		runningAt:   time.Now(),
	}
	hub.Use(Filter(hub))
	hub.Use(AckMiddleware(hub))
	go hub.Run()
	go hub.resendPendingMessages()
	return hub
}

func (h *Hub) Run() {
	defer func() {
		if r := recover(); r != nil {
			h.service.Logger().Error("hub panic")
		}
	}()
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.id] = client
			h.mu.Unlock()
			h.service.Logger().Info("User connected to websocket", zap.Uint("id", client.id))
			go func(id uint) {
				messages, err := h.service.Cache().GetOfflineMessages(id)
				if err != nil {
					return
				}
				for _, message := range messages {
					var msg *ChatMsg
					if err := json.Unmarshal([]byte(message), &msg); err != nil {
						h.service.Logger().Warn("Failed to get cache message: invalid JSON", zap.Error(err))
						continue
					}
					ctx := &MessageContext{Message: msg, Pending: true, To: []uint{id}}
					if h.sendDirect(ctx) {
						h.service.Cache().RemoveOfflineMessage(id, message)
					}
				}
			}(client.id)
		case client := <-h.unregister:
			h.mu.Lock()
			delete(h.clients, client.id)
			h.mu.Unlock()
		case message := <-h.chat:
			ctx := &MessageContext{Message: message, Cache: true, Pending: true, To: []uint{message.To}}
			h.Send(ctx)
		case message := <-h.broadcast:
			uid, ok := message.Extra.([]uint)
			if !ok {
				continue
			}
			message.Extra = nil
			ctx := &MessageContext{Message: message, Cache: true, Pending: true, To: uid}
			h.Send(ctx)
		case <-h.done:
			close(h.register)
			close(h.unregister)
			close(h.chat)
			close(h.broadcast)
			h.service.Logger().Info("Hub Stoped...")
			return
		}
	}
}

func (h *Hub) Stop() {
	h.closed.Store(true)
	h.mu.Lock()
	for _, c := range h.clients {
		c.closed = true
		c.send <- CloseSignal{}
	}
	h.mu.Unlock()

	for {
		h.mu.RLock()
		if len(h.clients) == 0 {
			h.mu.RUnlock()
			break
		}
		h.mu.RUnlock()
		time.Sleep(time.Millisecond * 50)
	}
	close(h.done)
}
func (h *Hub) Kick(id uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c := h.clients[id]
	if c == nil {
		return
	}
	c.closed = true
	c.send <- CloseSignal{}
}

func (h *Hub) IsClosed() bool {
	return h.closed.Load()
}

func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) Uptime() time.Duration {
	return time.Since(h.runningAt)
}

func (h *Hub) Register(c *Client) {
	h.register <- c
}

func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}

func (h *Hub) SendToChat(message *ChatMsg) {
	h.chat <- message
}

func (h *Hub) SendToBroadcast(message *ChatMsg) {
	if message.Extra == nil {
		uid, err := h.service.Cache().GetMembersAndCache(message.To)
		if err != nil {
			return
		}
		message.Extra = uid
	}

	h.broadcast <- message
}

func (h *Hub) SendToAck(message *AckMsg) {
	h.service.Cache().RemovePendingMessage(message.ID, message.To, message.Time)
}

func (h *Hub) SendUpdateBlockedListNotify(message *ChatMsg) {
	ctx := &MessageContext{Message: message, Pending: true, To: []uint{message.To}}
	h.Send(ctx)
}

func (h *Hub) SendToApply(message *ChatMsg) {
	uid, err := h.service.Cache().GetAdmin(message.To)
	if err != nil {
		return
	}
	message.Extra = uid
	h.broadcast <- message
}

// 返回值只对单发有效
func (h *Hub) sendDirect(ctx *MessageContext) bool {
	for _, id := range ctx.To {
		h.mu.RLock()
		c, ok := h.clients[id]
		if ok && c.conn != nil {
			// 群发下值复制避免后续迭代影响消息
			var msgCopy any
			if msg, ok := ctx.Message.(*ChatMsg); ok && msg.Type == Broadcast {
				msgCopy = &ChatMsg{
					ID:    msg.ID,
					Type:  msg.Type,
					From:  msg.From,
					To:    id,
					Body:  msg.Body,
					Time:  msg.Time,
					Extra: msg.To,
				}
			} else {
				// 单发不受影响
				msgCopy = ctx.Message
			}
			c.send <- msgCopy
			h.mu.RUnlock()
			/* 挂起消息
			第一次发送必定会标为true
			重发不会进入这个条件,只会缓存成离线消息后或者收到ack消息再删除挂起消息 */
			if ctx.Pending {
				// 统一使用指针
				if msg, ok := msgCopy.(*ChatMsg); ok {
					h.service.Cache().StorePendingMessage(msgCopy, msg.Time)
				}
				ctx.Sent = true
			}
			// 缓存成离线消息，视为发送成功
		} else if ctx.Cache {
			h.mu.RUnlock()
			h.StoreOfflineMessage(ctx.Message, id)
			ctx.Sent = true
		} else { // 应视为用户离线
			ctx.Sent = true
		}
	}
	return ctx.Sent
}

// 返回值只对单发有效
func (h *Hub) Send(ctx *MessageContext) bool {
	finalHandler := func(ctx *MessageContext) {
		h.sendDirect(ctx)
	}
	handler := finalHandler
	for i := len(h.middlewares) - 1; i >= 0; i-- {
		middleware := h.middlewares[i]
		next := handler
		handler = func(ctx *MessageContext) {
			middleware.Process(ctx, next)
		}
	}

	handler(ctx)
	return ctx.Sent
}

func (h *Hub) StoreOfflineMessage(message any, id uint) {
	h.service.Cache().StoreOfflineMessage(id, message)
}

func (h *Hub) Use(middleware ...MessageMiddleware) {
	h.middlewares = append(h.middlewares, middleware...)
}

type MessageMiddleware interface {
	Process(ctx *MessageContext, next func(ctx *MessageContext))
}

type ackMiddleware struct {
	hub *Hub
}

func AckMiddleware(hub *Hub) MessageMiddleware {
	return &ackMiddleware{
		hub: hub,
	}
}

func (m *ackMiddleware) Process(ctx *MessageContext, next func(ctx *MessageContext)) {
	msg, ok := ctx.Message.(*ChatMsg)
	if !ok {
		return
	}

	if msg.Time == 0 {
		msg.Time = time.Now().UnixMilli()
	}
	if msg.ID == "" {
		msg.ID = uuid.NewString()
	}

	next(ctx)
	if msg.Type != Chat && msg.Type != Broadcast { // 服务器生成的消息不需要确认
		return
	}

	ack := &AckMsg{
		Type: Ack,
		ID:   msg.ID,
		To:   msg.From,
	}
	ackCtx := &MessageContext{
		Message: ack,
		To:      []uint{ack.To},
		Cache:   true,
	}
	m.hub.sendDirect(ackCtx)
}

func (h *Hub) resendPendingMessages() {
	ticker := time.NewTicker(h.service.Config().Common().CheckAckTimeout())
	defer ticker.Stop()
	type header struct {
		Type int `json:"type"`
	}
	for {
		select {
		case <-ticker.C:
			// 重置计时器,应用热更新
			resendDelay := h.service.Config().Common().CheckAckTimeout()
			ticker.Reset(resendDelay)

			msgs, _ := h.service.Cache().GetPendingMessages()
			if len(msgs) == 0 {
				time.Sleep(resendDelay + time.Second)
				continue
			}
			for _, msg := range msgs {
				var t header
				if err := json.Unmarshal([]byte(msg), &t); err != nil {
					h.service.Logger().Error("Failed to parse message header", zap.String("msg", msg), zap.Error(err))
					continue
				}
				var err error
				switch t.Type {
				case Chat, System:
					var m *ChatMsg
					err = json.Unmarshal([]byte(msg), &m)
					if err == nil {
						ctx := &MessageContext{Message: m, Cache: true, To: []uint{m.To}}
						if h.sendDirect(ctx) {
							h.service.Cache().RemovePendingMessage(m.ID, m.To, m.Time)
						}
					}
				case Broadcast:
					var m *ChatMsg
					err = json.Unmarshal([]byte(msg), &m)
					if err == nil {
						to, ok := m.Extra.(float64)
						if !ok {
							continue
						}
						ctx := &MessageContext{Message: m, Cache: true, To: []uint{uint(to)}}
						if h.sendDirect(ctx) {
							h.service.Cache().RemovePendingMessage(m.ID, uint(to), m.Time)
						}
					}
				case UpdateBlackList:
					var m *ChatMsg
					err = json.Unmarshal([]byte(msg), &m)
					if err == nil {
						ctx := &MessageContext{Message: m, To: []uint{m.To}}
						if h.sendDirect(ctx) {
							h.service.Cache().RemovePendingMessage(m.ID, m.To, m.Time)
						}
					}
				default:
					h.service.Logger().Error("Unknown resend message type", zap.String("msg", msg))
					continue
				}
				if err != nil {
					h.service.Logger().Error("Unknown resend message type", zap.String("msg", msg))
				}
			}
			// 当前时间区间还有待重发消息，立即执行
			if int64(len(msgs)) > h.service.Config().Common().ResendBatchSize() {
				ticker.Reset(time.Millisecond)
			}
		case <-h.done:
			return
		}
	}
}

type filter struct {
	hub *Hub
}

func Filter(hub *Hub) MessageMiddleware {
	return &filter{hub}
}

func (f *filter) Process(ctx *MessageContext, next func(ctx *MessageContext)) {
	msg, ok := ctx.Message.(*ChatMsg)
	if !ok {
		return
	}

	to := ctx.To
	if msg.Type == Chat || msg.Type == Broadcast { // 只拦截用户发出的消息
		if f.hub.service.Cache().BFM().IsMuted(msg.From) &&
			f.hub.service.Cache().IsBanMuted(msg.From) {
			return
		}
	}

	j := len(to) - 1
	for i := 0; i <= j; i++ {
		for j >= 0 && f.banned(to[j]) {
			j--
		}
		if i < j && f.banned(to[i]) {
			to[i], to[j] = to[j], to[i]
		}
	}
	to = to[:j+1]
	ctx.To = to
	next(ctx)
}

func (f *filter) banned(id uint) bool {
	return f.hub.service.Cache().BFM().IsBanned(id) &&
		f.hub.service.Cache().IsBanPermanent(id)
}
