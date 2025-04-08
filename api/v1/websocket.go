package v1

import (
	"net/http"

	"github.com/farnese17/chat/registry"
	"github.com/farnese17/chat/websocket"
	"github.com/gin-gonic/gin"
)

func WsRoutes(c *gin.Context) {
	s := registry.GetService()
	if s.Hub() == nil || s.Hub().IsClosed() {
		c.JSON(http.StatusServiceUnavailable, nil)
		c.Abort()
		return
	}
	id := c.MustGet("from").(uint)
	websocket.UpgradeToWS(s, id, c.Writer, c.Request)
}
