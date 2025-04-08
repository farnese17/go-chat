package v1_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/farnese17/chat/middleware"
	"github.com/farnese17/chat/repository"
	"github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/errorsx"
	ws "github.com/farnese17/chat/websocket"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

var (
	server         *http.Server
	clients        = make(map[uint]*websocket.Conn)
	mu             = sync.RWMutex{}
	testCacheCount = 100
	port           int
)

func startWebsocket() {
	port = rand.IntN(1000) + 10000
	server = &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: route}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			fmt.Println(err)
		}
	}()
	if !s.Hub().IsClosed() {
		s.Hub().Stop()
	}
	s.SetHub(ws.NewHubInterface(s))
}

func shutdownWebsocket() {
	clearClients()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		fmt.Println(err)
	}
}

func registerClientToWs(t *testing.T, id uint) {
	t.Run(fmt.Sprintf("register %d", id), func(t *testing.T) {
		token, err := s.Cache().GetToken(id)
		if err != nil || token == "" {
			token, _ = middleware.GenerateToken(id)
			s.Cache().SetToken(id, token, s.Config().Common().TokenValidPeriod())
			s.Cache().Flush()
		}
		url := fmt.Sprintf("ws://localhost:%d/api/v1/ws", port)
		conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{
			"Authorization": []string{"Bearer " + token}})
		assert.NoError(t, err)
		mu.Lock()
		clients[id] = conn
		mu.Unlock()
	})
}

func TestRegisterClientToWs(t *testing.T) {
	startWebsocket()
	defer shutdownWebsocket()
	for i := range testDataCount {
		id := uint(1e5) + uint(i+1)
		go registerClientToWs(t, id)
	}

	waitingForClientsRegisterComplete(t, testDataCount)
	assert.Equal(t, testDataCount, s.Hub().Count())
}

func TestSentCacheMessage(t *testing.T) {
	startWebsocket()
	clearWebsocket()
	defer shutdownWebsocket()

	registerClientToWs(t, 100001)
	waitingForClientsRegisterComplete(t, 1)
	client := clients[100001]

	for i := 0; i < testCacheCount; i++ { // 发送离线消息
		for i := 1; i < testDataCount; i++ {
			msg := ws.ChatMsg{
				Type: ws.Chat,
				From: 100001,
				Body: "abcd",
				Time: time.Now().UnixNano(),
				To:   uint(1e5) + uint(i+1)}
			send(t, ws.Chat, msg, client)
		}
	}

	// 等待缓存完成
	id := make([]uint, testDataCount)
	for i := 1; i < testDataCount; i++ {
		id[i] = uint(1e5) + uint(i+1)
	}
	if !waitingForCacheComplete(testCacheCount, id[1:]) {
		t.Error("cache not ready")
	}

	// 注册客户端，获取离线消息
	wg := sync.WaitGroup{}
	for i := 1; i < testDataCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := uint(1e5) + uint(i+1)
			t.Run(fmt.Sprintf("resend cache message %d", id), func(t *testing.T) {
				count := testCacheCount
				registerClientToWs(t, id)
				conn := getConn(id)
				expected := ws.ChatMsg{
					Type: ws.Chat,
					From: 100001,
					Body: "abcd",
					To:   id}
				conn.SetReadDeadline(time.Now().Add(time.Second * 10))
				for {
					receivingChatMessage(t, conn, expected)
					count-- // 计数
					if count == 0 {
						break
					}
				}
				assert.Equalf(t, 0, count, "Expected %d messages, but %d got %d", testCacheCount, id, testCacheCount-count)
			})
		}()
	}
	wg.Wait()

	// 等待缓存清空
	if !waitingForCacheComplete(0, id[1:]) {
		t.Error("cache not clear")
	}
}

func TestWebsocketChat(t *testing.T) {
	startWebsocket()
	clearWebsocket()
	defer shutdownWebsocket()

	for i := range testDataCount {
		id := uint(1e5) + uint(i+1)
		go registerClientToWs(t, id)
	}

	waitingForClientsRegisterComplete(t, testDataCount)

	wg := sync.WaitGroup{}
	// start receive message
	for id, conn := range clients {
		wg.Add(1)
		go func(id uint, count int) {
			defer wg.Done()
			t.Run(fmt.Sprintf("%d receive message", id), func(t *testing.T) {
				expected := ws.ChatMsg{
					Type: ws.Chat,
					Body: "abcd",
					To:   id,
				}
				conn.SetReadDeadline(time.Now().Add(time.Second * 10))
				for {
					receivingChatMessage(t, conn, expected)
					count--
					if count == 0 {
						break
					}
				}
				assert.Equal(t, 0, count)
			})
		}(id, testDataCount-1)
	}

	for from, conn := range clients {
		wg.Add(1)
		msg := ws.ChatMsg{
			Type: ws.Chat,
			From: from,
			Body: "abcd",
			Time: time.Now().UnixMilli(),
		}
		go func(conn *websocket.Conn, msg ws.ChatMsg) {
			defer wg.Done()
			t.Run(fmt.Sprintf("%d send message", from), func(t *testing.T) {
				for to := range clients {
					if from == to {
						continue
					}
					msg.To = to
					send(t, ws.Chat, msg, conn)
				}
			})
		}(conn, msg)
	}
	wg.Wait()
}

func TestWebsocketBroadcast(t *testing.T) {
	startWebsocket()
	clearWebsocket()
	defer shutdownWebsocket()

	receiver := make([]uint, testDataCount)
	for i := range testDataCount {
		id := uint(1e5+1) + uint(i)
		go registerClientToWs(t, id)
		receiver[i] = id
	}
	waitingForClientsRegisterComplete(t, testDataCount)
	for _, id := range receiver {
		s.Cache().AddMember(uint(1e9+1), id, model.GroupRoleMember)
	}
	s.Cache().Flush()

	expected := ws.ChatMsg{
		Type: ws.Broadcast,
		From: 100001,
		To:   uint(1e9 + 1),
		Time: time.Now().UnixMilli(),
		Body: "abcd",
	}
	wg := sync.WaitGroup{}
	for id, conn := range clients {
		wg.Add(1)
		go func(conn *websocket.Conn, count int) {
			defer wg.Done()
			t.Run(fmt.Sprintf("%d receiving broadcast message", id), func(t *testing.T) {
				conn.SetReadDeadline(time.Now().Add(time.Second * 10))
				for {
					receivingChatMessage(t, conn, expected)
					count--
					if count == 0 {
						break
					}
				}
				assert.Equal(t, 0, count)
			})
		}(conn, len(clients)/10)
	}

	for range len(clients) / 10 {
		t.Run("send broadcast message", func(t *testing.T) {
			send(t, ws.Broadcast, expected, clients[100001])
		})
	}
	wg.Wait()
}

func TestReturnUnack(t *testing.T) {
	startWebsocket()
	clearWebsocket()
	defer shutdownWebsocket()

	id := uint(1e5 + 1)
	registerClientToWs(t, id)
	waitingForClientsRegisterComplete(t, 1)

	msg := ws.ChatMsg{
		Type: ws.Broadcast,
		From: id,
		To:   id + 1,
		Time: time.Now().UnixMilli(),
	}

	conn := clients[id]
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		expected := ws.ChatMsg{
			Type: ws.Ack,
			From: id,
			To:   id,
			Time: msg.Time,
			Data: false,
		}
		t.Run("send successed,but got ack message", func(t *testing.T) {
			_, p, err := conn.ReadMessage()
			assert.NoError(t, err)
			var result ws.Message
			json.Unmarshal(p, &result)
			var body ws.ChatMsg
			json.Unmarshal(result.Body, &body)
			assert.Equal(t, expected, body)
		})
	}()

	s.Hub().SendDeleteGroupNotify(&msg)
	wg.Wait()
}

func TestWebsocketStop(t *testing.T) {
	startWebsocket()
	clearCacheMessage()
	defer shutdownWebsocket()

	for i := range testDataCount {
		id := uint(1e5+1) + uint(i)
		go registerClientToWs(t, id)
	}
	waitingForClientsRegisterComplete(t, testDataCount)

	wg := sync.WaitGroup{}
	for id, conn := range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t.Run(fmt.Sprintf("%d start read", id), func(t *testing.T) {
				_, _, err := conn.ReadMessage()
				expected := websocket.CloseError{
					Code: websocket.CloseServiceRestart,
					Text: errorsx.ErrServerClosed.Error(),
				}
				assert.Equal(t, &expected, err)
			})
		}()
	}

	s.Hub().Stop()
	wg.Wait()
}

func TestSendUpdateNotify(t *testing.T) {
	startWebsocket()
	clearWebsocket()
	defer shutdownWebsocket()

	var sender uint = 100001

	for i := range testDataCount {
		id := uint(1e5+1) + uint(i)
		go registerClientToWs(t, id)
	}
	waitingForClientsRegisterComplete(t, testDataCount)

	expected := ws.HandleBlockMsg{
		Type:  ws.HandleBlock,
		From:  sender,
		Block: true,
	}
	wg := sync.WaitGroup{}
	for id, conn := range clients {
		wg.Add(1)
		go func(id uint, conn *websocket.Conn, expected ws.HandleBlockMsg) {
			defer wg.Done()
			expected.To = id
			t.Run(fmt.Sprintf("%d receive notify", id), func(t *testing.T) {
				conn.SetReadDeadline(time.Now().Add(time.Second * 10))
				_, p, err := conn.ReadMessage()
				assert.NoError(t, err)
				var message ws.Message
				json.Unmarshal(p, &message)
				var msg ws.HandleBlockMsg
				json.Unmarshal(message.Body, &msg)
				assert.NotEmpty(t, msg.Time)
				expected.Time = msg.Time
				assert.Equal(t, expected, msg)
			})
		}(id, conn, expected)
	}

	for id := range clients {
		msg := expected
		msg.To = id
		s.Hub().SendUpdateBlockedListNotify(&msg)
	}
	wg.Wait()
}

func TestACkMessage(t *testing.T) {
	startWebsocket()
	clearWebsocket()
	defer shutdownWebsocket()

	// register and set cache
	cache := s.Cache().(repository.BlockCache)
	for i := range testDataCount {
		id := uint(1e5+1) + uint(i)
		go registerClientToWs(t, id)
		msg := ws.HandleBlockMsg{
			Type: ws.HandleBlock,
			To:   id,
		}
		cache.CacheUnackMessage(time.Duration(time.Now().Add(time.Minute).UnixMilli()), msg)
	}
	s.Cache().Flush()
	waitingForClientsRegisterComplete(t, testDataCount)

	// send ack message
	for id, conn := range clients {
		body, _ := json.Marshal(ws.HandleBlockMsg{
			Type: ws.HandleBlock,
			To:   id,
			Ack:  true})

		msg := ws.Message{
			Type: ws.HandleBlock,
			Body: body,
		}
		err := conn.WriteJSON(msg)
		assert.NoError(t, err)
	}

	// waiting for remove cache
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
outerLook:
	for {
		select {
		case <-ticker.C:
			t.Error("waiting for remove cache timeout")
			break outerLook
		default:
			msgs, err := cache.GetUnAck()
			assert.NoError(t, err)
			if len(msgs) == 0 {
				break outerLook
			}
			time.Sleep(time.Millisecond * 20)
		}
	}
}

func TestResendUnackMessage(t *testing.T) {
	startWebsocket()
	clearWebsocket()
	defer shutdownWebsocket()

	cache := s.Cache().(repository.BlockCache)
	for i := range testDataCount {
		id := uint(1e5+1) + uint(i)
		go registerClientToWs(t, id)
		msg := ws.HandleBlockMsg{
			Type: ws.HandleBlock,
			To:   id,
		}
		cache.CacheUnackMessage(0, msg)
	}
	s.Cache().Flush()
	waitingForClientsRegisterComplete(t, testDataCount)

	wg := sync.WaitGroup{}
	for id, conn := range clients {
		wg.Add(1)
		go func(id uint, conn *websocket.Conn) {
			defer wg.Done()
			expected := ws.HandleBlockMsg{
				Type: ws.HandleBlock,
				To:   id,
			}
			t.Run(fmt.Sprintf("%d receive resend notify", id), func(t *testing.T) {
				conn.SetReadDeadline(time.Now().Add(time.Second * 10))
				_, p, err := conn.ReadMessage()
				assert.NoError(t, err)
				var message ws.Message
				json.Unmarshal(p, &message)
				var msg ws.HandleBlockMsg
				json.Unmarshal(message.Body, &msg)
				assert.Equal(t, expected, msg)
			})
		}(id, conn)
	}

	wg.Wait()
}

func send(t *testing.T, msgType int, msg any, conn *websocket.Conn) {
	body, _ := json.Marshal(msg)
	message := ws.Message{
		Type: msgType,
		Body: body,
	}
	err := conn.WriteJSON(message)
	assert.NoError(t, err)
}

func receivingChatMessage(t *testing.T, conn *websocket.Conn, expected ws.ChatMsg) {
	_, p, err := conn.ReadMessage()
	assert.NoError(t, err)
	var message ws.Message
	json.Unmarshal(p, &message)
	var msg ws.ChatMsg
	json.Unmarshal(message.Body, &msg)
	fixMessageTime(t, &expected, &msg)
	fixExecptedID(t, &expected, &msg)
	assert.Equal(t, expected, msg)
}

func fixExecptedID(t *testing.T, expected, msg *ws.ChatMsg) {
	if expected.From == 0 {
		assert.NotEmpty(t, msg.From)
		expected.From = msg.From
	}
}
func fixMessageTime(t *testing.T, expected, msg *ws.ChatMsg) {
	if expected.Time == 0 {
		assert.NotEmpty(t, msg.Time)
		expected.Time = msg.Time
	}
}

func getConn(id uint) *websocket.Conn {
	mu.RLock()
	defer mu.RUnlock()
	return clients[id]
}

func waitingForClientsRegisterComplete(t *testing.T, count int) {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.Error("waiting for client register timeout")
			return
		default:
			if count == s.Hub().Count() {
				return
			}
			time.Sleep(time.Millisecond * 20)
		}
	}
}

func waitingForCacheComplete(count int, id []uint) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i := range id {
	outerLoop:
		for {
			select {
			case <-ctx.Done():
				return false
			default:
				if checkCacheStatus(count, id[i]) {
					break outerLoop
				}
				time.Sleep(time.Millisecond * 20)
			}
		}
	}
	return true
}

func checkCacheStatus(count int, id uint) bool {
	cache := s.Cache().(repository.TestableCache)
	key := model.CacheMessage + strconv.FormatUint(uint64(id), 10)
	c := cache.CountSet(key)
	return c == int64(count)
}

func checkCache(id []uint) {
	s.Cache().Flush()
	cache := s.Cache().(repository.TestableCache)
	for i := range id {
		key := model.CacheMessage + strconv.FormatUint(uint64(id[i]), 10)
		count := cache.CountSet(key)
		fmt.Printf("id: %d, count: %d\n", id[i], count)
	}
}

func clearWebsocket() {
	clearCacheMessage()
	clearWaitingAckMessage()
}

func clearCacheMessage() {
	for i := range testDataCount {
		id := uint(1e5) + uint(i+1)
		key := model.CacheMessage + strconv.FormatUint(uint64(id), 10)
		s.Cache().Remove(key)
	}
	s.Cache().Flush()
}

func clearClients() {
	mu.Lock()
	defer mu.Unlock()
	// for _, conn := range clients {
	// 	conn.Close()
	// }
	clients = make(map[uint]*websocket.Conn)
}

func clearWaitingAckMessage() {
	cache := s.Cache().(repository.BlockCache)
	messages, err := cache.GetUnAck()
	if err != nil {
		panic(err)
	}
	for _, m := range messages {
		cache.RemoveBlockMessage(m)
	}
	s.Cache().Flush()
}
