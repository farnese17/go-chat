package v1

import (
	"github.com/farnese17/chat/registry"
	"github.com/farnese17/chat/service"
	"github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/ginx"
	"github.com/gin-gonic/gin"
)

var f *service.FriendService

func SetupFriendService(s registry.Service) {
	f = service.NewFriendService(s)
}

func Request(c *gin.Context) {
	handleFriendStatus(c, f.Request)
}

func Accept(c *gin.Context) {
	handleFriendStatus(c, f.Accept)
}

func Reject(c *gin.Context) {
	handleFriendStatus(c, f.Reject)
}

func RemoveFriend(c *gin.Context) {
	handleFriendStatus(c, f.Delete)
}

func BlockFriend(c *gin.Context) {
	handleFriendStatus(c, f.Block)
}

func UnblockFriend(c *gin.Context) {
	handleFriendStatus(c, f.Unblock)
}

func handleFriendStatus(c *gin.Context, fn func(uint, uint) error) {
	from := ginx.GetUserID(c)
	to := c.MustGet("to").(uint)
	ginx.NoDataResponse(c, func() error {
		return fn(from, to)
	})
}

func FriendList(c *gin.Context) {
	from := ginx.GetUserID(c)
	ginx.HasDataResponse(c, func() (any, error) {
		return f.List(from)
	})
}
func BlockedMeList(c *gin.Context) {
	from := ginx.GetUserID(c)
	ginx.HasDataResponse(c, func() (any, error) {
		return f.BlockedMeList(from)
	})
}

func SearchFriend(c *gin.Context) {
	from := ginx.GetUserID(c)
	value := c.Query("value")
	var cursor *model.Cursor
	c.ShouldBindJSON(&cursor)
	ginx.HasDataResponse(c, func() (any, error) {
		return f.Search(from, value, cursor)
	})
}

func GetFriend(c *gin.Context) {
	from := ginx.GetUserID(c)
	to := c.MustGet("to").(uint)
	ginx.HasDataResponse(c, func() (any, error) {
		return f.Get(from, to)
	})
}

func SetRemark(c *gin.Context) {
	from := ginx.GetUserID(c)
	to := c.MustGet("to").(uint)
	value := c.Query("remark")
	ginx.NoDataResponse(c, func() error {
		return f.Remark(from, to, value)
	})
}

func SetGroup(c *gin.Context) {
	from := ginx.GetUserID(c)
	to := c.MustGet("to").(uint)
	value := c.Query("group")
	ginx.NoDataResponse(c, func() error {
		return f.SetGroup(from, to, value)
	})
}
