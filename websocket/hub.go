package websocket

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/farnese17/chat/config"
	repo "github.com/farnese17/chat/repository"
	"go.uber.org/zap"
)

type Service interface {
	Logger() *zap.Logger
	Config() config.Config
	Cache() repo.Cache
	Hub() HubInterface
}

type HubInterface interface {
	Run()
	Stop()
	Register(c *Client)
	Unregister(c *Client)
	SendToChat(message *ChatMsg)
	SendToBroadcast(message *ChatMsg)
	SendToApply(message *ChatMsg)
	SendDeleteGroupNotify(message *ChatMsg)
	SendUpdateBlockedListNotify(message *HandleBlockMsg)
	CacheMessage(message any, id uint)
	Count() int
	IsClosed() bool
	Kick(id uint)
}

func NewHubInterface(service Service) HubInterface {
	return NewHub(service)
}

// var hub *Hub

const checkTimeoutMsgInterval = 500 * time.Millisecond

type Hub struct {
	clients    map[uint]*Client
	register   chan *Client
	unregister chan *Client
	chat       chan *ChatMsg
	broadcast  chan *ChatMsg
	done       chan struct{}
	closed     atomic.Bool
	mu         sync.RWMutex
	blocked    *blockedList
	service    Service
}

func NewHub(service Service) *Hub {
	hub := &Hub{
		clients:    make(map[uint]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		chat:       make(chan *ChatMsg),
		broadcast:  make(chan *ChatMsg),
		done:       make(chan struct{}),
		closed:     atomic.Bool{},
		mu:         sync.RWMutex{},
		blocked:    newBlockedList(service),
		service:    service,
	}
	hub.Run()
	return hub
}

func (h *Hub) Run() {
	go func() {
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
					messages, err := h.service.Cache().GetMessage(id)
					if err != nil {
						return
					}
					for _, message := range messages {
						var msg *ChatMsg
						if err := json.Unmarshal([]byte(message), &msg); err != nil {
							h.service.Logger().Warn("Failed to get cache message: invalid JSON", zap.Error(err))
							continue
						}
						if h.Send(id, msg, false) {
							h.service.Cache().RemoveMessage(id, message)
						}
					}
				}(client.id)
			case client := <-h.unregister:
				h.mu.Lock()
				delete(h.clients, client.id)
				h.mu.Unlock()
			case message := <-h.chat:
				h.Send(message.To, message, true)
			case message := <-h.broadcast:
				uid, ok := message.Data.([]uint)
				if !ok {
					message.Type = Ack
					message.Data = false
					message.To = message.From
					h.Send(message.To, message, true)
					continue
				}
				message.Data = nil
				for _, id := range uid {
					if h.service.Cache().BFM().IsBanned(id) &&
						h.service.Cache().IsBanPermanent(id) {
						continue
					}
					h.Send(id, message, true)
				}
			case <-h.done:
				close(h.register)
				close(h.unregister)
				close(h.chat)
				close(h.broadcast)
				h.service.Logger().Info("Hub Stoped...")
				return
			}
		}
	}()

	go h.HandleBlock()
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
	// hub = nil
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
func (h *Hub) Register(c *Client) {
	h.register <- c
}
func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}
func (h *Hub) SendToChat(message *ChatMsg) {
	if h.service.Cache().BFM().IsBanned(message.To) &&
		h.service.Cache().IsBanPermanent(message.To) {
		return
	}
	if h.service.Cache().BFM().IsMuted(message.From) &&
		h.service.Cache().IsBanMuted(message.From) {
		return
	}
	h.chat <- message
}
func (h *Hub) SendToBroadcast(message *ChatMsg) {
	if h.service.Cache().BFM().IsMuted(message.From) &&
		h.service.Cache().IsBanMuted(message.From) {
		return
	}
	uid, err := h.service.Cache().GetMembersAndCache(message.To)
	if err != nil { // 返回消息未送达
		message.Type = Ack
		message.Data = false
		message.To = message.From
		h.Send(message.To, message, true)
		return
	}
	message.Data = uid
	h.broadcast <- message
}
func (h *Hub) SendUpdateBlockedListNotify(message *HandleBlockMsg) {
	if message.Ack {
		h.blocked.ack <- message
	} else {
		h.blocked.sendNotify <- message
	}
}

func (h *Hub) SendToApply(message *ChatMsg) {
	uid, err := h.service.Cache().GetAdmin(message.To)
	if err != nil {
		return
	}
	message.Data = uid
	h.broadcast <- message
}
func (h *Hub) SendDeleteGroupNotify(message *ChatMsg) {
	h.broadcast <- message
}

type blockedList struct {
	sendNotify chan *HandleBlockMsg
	ack        chan *HandleBlockMsg
	cache      repo.BlockCache
}

func newBlockedList(service Service) *blockedList {
	return &blockedList{
		sendNotify: make(chan *HandleBlockMsg),
		ack:        make(chan *HandleBlockMsg),
		cache:      service.Cache().(repo.BlockCache),
	}
}

// 更新前端维护的黑名单。
// 第一层过滤通过发送方的前端通过维护一个“我被谁拉黑”名单，
// 以及一个用户对的最后更新时间用于筛选过期消息确保数据一致性。
// 第二层过滤通过接收方的本地黑名单拦截
func (h *Hub) HandleBlock() {
	ticker := time.NewTicker(checkTimeoutMsgInterval)
	defer ticker.Stop()
	defer func() {
		if r := recover(); r != nil {
			h.service.Logger().Error("update blacklist panic")
		}
	}()

	for {
		select {
		case message := <-h.blocked.sendNotify:
			message.Time = time.Now().UnixNano()                                                     // 使用纳秒保证消息的顺序
			h.blocked.cache.CacheUnackMessage(h.service.Config().Common().ResendInterval(), message) // 缓存消息等待前端确认，用于重试
			h.Send(message.To, message, false)                                                       // 发送消息通知被拉黑方前端更新
			h.service.Logger().Info("Updating blacklist")
		case message := <-h.blocked.ack:
			message.Ack = false                                    // 缓存一致
			h.blocked.cache.Ack(message.From, message.To, message) // 被拉黑方前端更新完成，删除缓存
			h.service.Logger().Info("Add Ack message")
		case <-ticker.C:
			// 获取过期消息
			data, err := h.blocked.cache.GetUnAck() // 定时重发通知
			if err != nil {
				continue
			}
			count := 0
			for _, m := range data {
				var message *HandleBlockMsg
				err := json.Unmarshal([]byte(m), &message)
				if err != nil {
					h.service.Logger().Error(err.Error())
					continue
				}
				if !h.Send(message.To, message, false) { // 用户离线，删除缓存
					h.blocked.cache.RemoveBlockMessage(m)
					count++
				}
			}
			if len(data) > 0 { // 日志记录
				h.service.Logger().Info(fmt.Sprintf("Get cache ack message count: total %d ,resend %d, invalid %d",
					len(data), len(data)-count, count))
			}
		case <-h.done:
			close(h.blocked.sendNotify)
			close(h.blocked.ack)
			h.service.Logger().Info("Blocked Manager shutting down...")
			return
		}
	}
}

func (h *Hub) Send(to uint, message any, cache bool) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.clients[to]
	if ok && c.conn != nil {
		c.send <- message
		return true
	} else if cache {
		h.CacheMessage(message, to)
	}
	return false
}

func (h *Hub) CacheMessage(message any, id uint) {
	h.service.Cache().CacheMessage(id, message)
}
