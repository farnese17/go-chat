package websocket

import (
	"fmt"
	"net/http"

	"github.com/farnese17/chat/utils/errorsx"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func UpgradeToWS(s Service, id uint, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade http to websocket", http.StatusInternalServerError)
		s.Logger().Error("Failed to upgrade http to websocket", zap.Error(err))
		fmt.Fprintf(w, "%v\n", err)
		return
	}

	client := NewWsClient(s, id, conn)
	if wsIsClosed(s) {
		client.sendCloseMessage()
		conn.Close()
		return
	}

	s.Hub().Register(client)
	go client.Read()
	go client.Write()
}

func StopWebsocket(s Service) error {
	if wsIsClosed(s) {
		return errorsx.ErrServerClosed
	}
	s.Hub().Stop()
	s.SetHub(nil)
	return nil
}

func wsIsClosed(s Service) bool {
	if s.Hub() == nil || s.Hub().IsClosed() {
		return true
	}
	return false
}
