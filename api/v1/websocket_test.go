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
	"github.com/google/uuid"
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

func TestSendOfflineMessages(t *testing.T) {
	startWebsocket()
	clearWebsocket()
	defer shutdownWebsocket()

	registerClientToWs(t, 100001)
	waitingForClientsRegisterComplete(t, 1)
	client := clients[100001]

	// 发送离线消息
	msgTime := genMsgTime()
	id := make([]uint, testDataCount)
	for i := 1; i < testDataCount; i++ {
		to := uint(1e5) + uint(i+1)
		id[i] = to
		for j := 0; j < testCacheCount; j++ {
			msg := ws.ChatMsg{
				Type: ws.Chat,
				From: 100001,
				Body: "abcd",
				Time: msgTime,
				To:   to,
			}
			send(t, ws.Chat, msg, client)
		}
	}
	s.Cache().Flush()

	// 等待缓存完成
	if !waitingForCacheComplete(testCacheCount, id[1:]) {
		t.Error("cache not ready")
	}

	// 注册客户端，获取离线消息
	wg := sync.WaitGroup{}
	for i := 1; i < testDataCount; i++ {
		wg.Add(1)
		go func(i int) {
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
					receiveChatMessage(t, conn, expected)
					count-- // 计数
					if count == 0 {
						break
					}
				}
				assert.Equalf(t, 0, count, "Expected %d messages, but %d got %d", testCacheCount, id, testCacheCount-count)
			})
		}(i)
	}
	wg.Wait()

	// 等待缓存离线消息清空
	if !waitingForCacheComplete(0, id[1:]) {
		t.Error("cache not clear")
	}

	// 接收ack消息
	t.Run("receive ack messages", func(t *testing.T) {
		count := testDataCount*testCacheCount - testCacheCount
		conn := clients[100001]
		expectedAck := ws.AckMsg{
			Type: ws.Ack,
			To:   100001,
		}
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		for i := 0; i < count; i++ {
			_, p, err := conn.ReadMessage()
			assert.NoError(t, err)
			var message ws.Message
			err = json.Unmarshal(p, &message)
			assert.NoError(t, err)
			assert.Equal(t, ws.Ack, message.Type)
			bodyData, err := json.Marshal(message.Body)
			assert.NoError(t, err)
			var msg ws.AckMsg
			err = json.Unmarshal(bodyData, &msg)
			assert.NoError(t, err)
			assert.NotEmpty(t, msg.ID)
			expectedAck.ID = msg.ID
			expectedAck.Time = msg.Time
			assert.Equal(t, expectedAck, msg)
		}
	})

	// 确保挂起消息不为空
	checkPendingMessage(t)
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
				expectedAck := ws.AckMsg{
					Type: ws.Ack,
					To:   id,
				}
				conn.SetReadDeadline(time.Now().Add(time.Second * 10))
				for {
					_, p, err := conn.ReadMessage()
					assert.NoError(t, err)
					var got ws.Message
					err = json.Unmarshal(p, &got)
					assert.NoError(t, err)
					if got.Type == ws.Chat {
						var msg ws.ChatMsg
						err := json.Unmarshal(got.Body, &msg)
						assert.NoError(t, err)
						assert.NotEmpty(t, msg.ID)
						assert.NotEmpty(t, msg.From)
						assert.NotEmpty(t, msg.Time)
						expected.ID = msg.ID
						expected.From = msg.From
						expected.Time = msg.Time
						assert.Equal(t, expected, msg)
					} else if got.Type == ws.Ack {
						var msg ws.AckMsg
						err := json.Unmarshal(got.Body, &msg)
						assert.NoError(t, err)
						assert.NotEmpty(t, msg.ID)
						expectedAck.ID = msg.ID
						assert.Equal(t, expectedAck, msg)
					} else {
						t.Errorf("received unexpected message: %s", string(p))
					}
					count--
					if count == 0 {
						break
					}
				}
				assert.Equal(t, 0, count)
			})
		}(id, (len(clients)-1)*2)
	}

	msgTime := genMsgTime()
	for from, conn := range clients {
		wg.Add(1)
		msg := ws.ChatMsg{
			Type: ws.Chat,
			From: from,
			Body: "abcd",
			Time: msgTime,
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
	s.Cache().Flush()
	checkPendingMessage(t)
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

	wg := sync.WaitGroup{}
	for id, conn := range clients {
		wg.Add(1)
		go func(id uint, conn *websocket.Conn, count int) {
			defer wg.Done()
			if id == 100001 {
				count *= 2
			}
			t.Run(fmt.Sprintf("%d receive broadcast message", id), func(t *testing.T) {
				expected := ws.ChatMsg{
					Type:  ws.Broadcast,
					From:  100001,
					To:    id,
					Body:  "abcd",
					Extra: 1e9 + 1,
				}
				expectedAck := ws.AckMsg{
					Type: ws.Ack,
					To:   id,
				}
				conn.SetReadDeadline(time.Now().Add(time.Second * 10))
				for {
					// receiveChatMessage(t, conn, expected)
					_, p, err := conn.ReadMessage()
					assert.NoError(t, err)
					var got ws.Message
					err = json.Unmarshal(p, &got)
					assert.NoError(t, err)
					if got.Type == ws.Broadcast {
						var msg ws.ChatMsg
						err := json.Unmarshal(got.Body, &msg)
						assert.NoError(t, err)
						assert.NotEmpty(t, msg.ID)
						assert.NotEmpty(t, msg.Time)
						expected.ID = msg.ID
						expected.Time = msg.Time
						assert.Equal(t, expected, msg)
					} else if got.Type == ws.Ack {
						var msg ws.AckMsg
						err := json.Unmarshal(got.Body, &msg)
						assert.NoError(t, err)
						assert.NotEmpty(t, msg.ID)
						expectedAck.ID = msg.ID
						assert.Equal(t, expectedAck, msg)
					} else {
						t.Errorf("received unexpected message: %s", string(p))
					}
					count--
					if count == 0 {
						break
					}
				}
				assert.Equal(t, 0, count)
			})
		}(id, conn, len(clients)/10)
	}

	t.Run("send broadcast message", func(t *testing.T) {
		msg := ws.ChatMsg{
			Type: ws.Broadcast,
			From: 100001,
			To:   uint(1e9 + 1),
			Body: "abcd",
			Time: genMsgTime(),
		}
		for range len(clients) / 10 {
			send(t, ws.Broadcast, msg, clients[100001])
		}
	})

	wg.Wait()

	checkPendingMessage(t)
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

	// 接收服务关闭通知
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

	expected := &ws.ChatMsg{
		Type:  ws.UpdateBlackList,
		From:  sender,
		Extra: true,
	}
	wg := sync.WaitGroup{}
	for id, conn := range clients {
		wg.Add(1)
		expected.To = id
		go func(id uint, conn *websocket.Conn, expected ws.ChatMsg) {
			defer wg.Done()
			t.Run(fmt.Sprintf("%d receive notify", id), func(t *testing.T) {
				conn.SetReadDeadline(time.Now().Add(time.Second * 10))
				_, p, err := conn.ReadMessage()
				assert.NoError(t, err)
				var message ws.Message
				json.Unmarshal(p, &message)
				var msg ws.ChatMsg
				json.Unmarshal(message.Body, &msg)
				assert.NotEmpty(t, msg.ID)
				assert.NotEmpty(t, msg.Time)
				expected.ID = msg.ID
				expected.Time = msg.Time
				assert.Equal(t, expected, msg)
			})
		}(id, conn, *expected)
	}

	msgTime := genMsgTime()
	for id := range clients {
		msg := ws.ChatMsg{
			Type:  ws.UpdateBlackList,
			From:  sender,
			To:    id,
			Extra: true,
			Time:  msgTime,
		}
		s.Hub().SendUpdateBlockedListNotify(&msg)
	}
	wg.Wait()

	checkPendingMessage(t)
}

func TestACkMessage(t *testing.T) {
	startWebsocket()
	clearWebsocket()
	defer shutdownWebsocket()

	var msgIDs []string
	msgTime := genMsgTime()
	// register and set cache
	for i := range testDataCount {
		id := uint(1e5+1) + uint(i)
		go registerClientToWs(t, id)
		msgid := uuid.NewString()
		msg := ws.ChatMsg{
			ID:   msgid,
			Type: ws.Chat,
			To:   id,
		}
		msgIDs = append(msgIDs, msgid)
		s.Cache().StorePendingMessage(msg, msgTime)
	}
	s.Cache().Flush()
	waitingForClientsRegisterComplete(t, testDataCount)

	// send ack message
	for i := range testDataCount {
		msgid := msgIDs[i]
		id := uint(1e5+1) + uint(i)
		body, _ := json.Marshal(ws.AckMsg{
			ID:   msgid,
			Type: ws.Ack,
			To:   id,
			Time: msgTime,
		})

		msg := ws.Message{
			Type: ws.Ack,
			Body: body,
		}
		conn := clients[id]
		err := conn.WriteJSON(msg)
		assert.NoError(t, err)
	}

	// waiting for remove pending messages
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

outerLoop:
	for {
		select {
		case <-ticker.C:
			t.Error("waiting for remove cache timeout")
			break outerLoop
		default:
			msgs, err := s.Cache().GetPendingMessages()
			assert.NoError(t, err)
			if len(msgs) == 0 {
				break outerLoop
			}
			time.Sleep(time.Millisecond * 20)
		}
	}
}

func TestResendPendingMessage(t *testing.T) {
	startWebsocket()
	clearWebsocket()
	defer shutdownWebsocket()

	// 调整重发间隔，减少测试时间
	s.Config().SetCommon("check_ack_timeout", "10ms")

	msgs := make(map[uint]ws.ChatMsg)
	msgTime := genMsgTime()
	for i := range testDataCount {
		id := uint(1e5+1) + uint(i)
		go registerClientToWs(t, id)
		msg := ws.ChatMsg{
			ID:   uuid.NewString(),
			Type: ws.Chat,
			To:   id,
			Time: msgTime,
		}
		msgs[id] = msg
		s.Cache().StorePendingMessage(msg, msgTime)
	}
	s.Cache().Flush()
	waitingForClientsRegisterComplete(t, testDataCount)

	wg := sync.WaitGroup{}
	for id, conn := range clients {
		wg.Add(1)
		go func(id uint, conn *websocket.Conn) {
			defer wg.Done()
			expected := msgs[id]
			t.Run(fmt.Sprintf("%d receive resend messages", id), func(t *testing.T) {
				count := 3
				for ; count > 0; count-- {
					conn.SetReadDeadline(time.Now().Add(time.Second * 10))
					_, p, err := conn.ReadMessage()
					assert.NoError(t, err)
					var message ws.Message
					json.Unmarshal(p, &message)
					var msg ws.ChatMsg
					json.Unmarshal(message.Body, &msg)
					assert.Equal(t, expected, msg)
				}
				assert.Zero(t, count)
			})
		}(id, conn)
	}
	wg.Wait()

	// 确保批大小足够
	s.Config().SetCommon("resend_batch_size", fmt.Sprintf("%d", testDataCount+1))
	data, err := s.Cache().GetPendingMessages()
	assert.NoError(t, err)
	assert.Equal(t, testDataCount, len(data))
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

func receiveChatMessage(t *testing.T, conn *websocket.Conn, expected ws.ChatMsg) {
	_, p, err := conn.ReadMessage()
	assert.NoError(t, err)
	var message ws.Message
	json.Unmarshal(p, &message)
	assert.Equal(t, expected.Type, message.Type)
	var msg ws.ChatMsg
	json.Unmarshal(message.Body, &msg)
	fixMessageID(t, &expected, &msg)
	fixMessageTime(t, &expected, &msg)
	fixExecptedID(t, &expected, &msg)
	assert.Equal(t, expected, msg)
}

func fixMessageID(t *testing.T, expected, msg *ws.ChatMsg) {
	if expected.ID == "" {
		assert.NotEmpty(t, msg.ID)
		expected.ID = msg.ID
	}
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

func clearWebsocket() {
	clearCacheMessage()
	clearPendingMessages()
}

func clearCacheMessage() {
	for i := range testDataCount {
		id := uint(1e5) + uint(i+1)
		key := fmt.Sprintf("%s%d", model.CacheMessage, id)
		s.Cache().Remove(key)
	}
	s.Cache().Flush()
}

func clearClients() {
	mu.Lock()
	defer mu.Unlock()
	clients = make(map[uint]*websocket.Conn)
}

func clearPendingMessages() {
	s.Cache().Remove(model.CacheMessagePending)
	s.Cache().Flush()
}

func checkPendingMessage(t *testing.T) {
	t.Run("pending messages not empty", func(t *testing.T) {
		s.Cache().Flush()
		msgs, err := s.Cache().GetPendingMessages()
		assert.NoError(t, err)
		assert.NotZero(t, len(msgs))
	})
}

func genMsgTime() int64 {
	return time.Now().Add(-5 * time.Second).UnixMilli()

}
