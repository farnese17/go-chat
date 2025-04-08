package v1

import (
	"strconv"

	"github.com/farnese17/chat/registry"
	"github.com/farnese17/chat/service"
	"github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/ginx"
	"github.com/farnese17/chat/websocket"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var g *service.GroupService
var logger *zap.Logger

func SetupGroupService(s registry.Service) {
	g = service.NewGroupService(s)
	logger = s.Logger()
}

func Create(c *gin.Context) {
	var group *model.Group
	c.ShouldBindJSON(&group)
	id := ginx.GetUserID(c)
	group.Owner = id
	group.Founder = id
	group.GID = 0
	group.CreatedAt = 0
	group.LastTime = 0
	ginx.HasDataResponse(c, func() (any, error) {
		return g.Create(group)
	})
}

func SearchByID(c *gin.Context) {
	gid, err := strconv.ParseUint(c.Param("gid"), 10, 64)
	if err != nil {
		logger.Warn("Failed to search group: bad gid", zap.String("gid", c.Query("gid")))
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.HasDataResponse(c, func() (any, error) {
		return g.SearchByID(uint(gid))
	})
}

func SearchByName(c *gin.Context) {
	name := c.Query("name")
	var cursor *model.Cursor
	c.ShouldBindJSON(&cursor)
	ginx.HasDataResponse(c, func() (any, error) {
		return g.SearchByName(name, cursor)
	})
}

func Invite(c *gin.Context) {
	from := ginx.GetUserID(c)
	to, err1 := strconv.ParseUint(c.Param("id"), 10, 64)
	gid, err2 := strconv.ParseUint(c.Param("gid"), 10, 64)
	if err1 != nil || err2 != nil {
		logger.Warn("Failed to invite user join to group: invaild param",
			zap.Uint("from", from),
			zap.String("to", c.Param("id")),
			zap.String("gid", c.Param("gid")))
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.HasDataResponse(c, func() (any, error) {
		return g.Invite(from, uint(to), uint(gid))
	})
}

func Apply(c *gin.Context) {
	id := ginx.GetUserID(c)
	gid, err := strconv.ParseUint(c.Param("gid"), 10, 64)
	if err != nil {
		logger.Warn("Failed to apply into group: invalid param",
			zap.Uint("id", id),
			zap.String("gid", c.Query("gid")))
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.NoDataResponse(c, func() error {
		return g.Apply(uint(gid), id)
	})
}

func Member(c *gin.Context) {
	id, iderr := strconv.ParseUint(c.Param("id"), 10, 64)
	gid, giderr := strconv.ParseUint(c.Param("gid"), 10, 64)
	if iderr != nil || giderr != nil {
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.HasDataResponse(c, func() (any, error) {
		return g.Member(uint(id), uint(gid))
	})
}

func Members(c *gin.Context) {
	gid, err := strconv.ParseUint(c.Param("gid"), 10, 64)
	if err != nil {
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.HasDataResponse(c, func() (any, error) {
		return g.Members(uint(gid))
	})
}

func Delete(c *gin.Context) {
	id := ginx.GetUserID(c)
	gid, err := strconv.ParseUint(c.Param("gid"), 10, 64)
	if err != nil {
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.NoDataResponse(c, func() error {
		return g.Delete(uint(gid), id)
	})
}

func Update(c *gin.Context) {
	from := ginx.GetUserID(c)
	gid, err := strconv.ParseUint(c.Param("gid"), 10, 64)
	if err != nil {
		ginx.HandleInvalidParam(c)
		return
	}
	field := c.Query("field")
	value := c.Query("value")
	ginx.NoDataResponse(c, func() error {
		return g.Update(from, uint(gid), field, value)
	})
}

func HandOverOwner(c *gin.Context) {
	from := ginx.GetUserID(c)
	to, err1 := strconv.ParseUint(c.Param("id"), 10, 64)
	gid, err2 := strconv.ParseUint(c.Param("gid"), 10, 64)
	if err1 != nil || err2 != nil {
		logger.Warn("Failed to change group owner: invaild param",
			zap.Uint("from", from),
			zap.String("to", c.Param("id")),
			zap.String("gid", c.Param("gid")))
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.NoDataResponse(c, func() error {
		return g.HandOverOwner(from, uint(to), uint(gid))
	})
}

func ModifyAdmin(c *gin.Context) {
	from := ginx.GetUserID(c)
	to, err1 := strconv.ParseUint(c.Param("id"), 10, 64)
	gid, err2 := strconv.ParseUint(c.Param("gid"), 10, 64)
	role, err3 := strconv.Atoi(c.Query("role"))
	if err1 != nil || err2 != nil || err3 != nil {
		logger.Warn("Failed to set admin: invaild param",
			zap.Uint("from", from),
			zap.String("to", c.Param("id")),
			zap.String("gid", c.Param("gid")))
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.NoDataResponse(c, func() error {
		return g.ModifyAdmin(from, uint(to), uint(gid), role)
	})
}

func AdminResign(c *gin.Context) {
	uid := ginx.GetUserID(c)
	gid, err := strconv.Atoi(c.Param("gid"))
	if err != nil {
		logger.Warn("Failed to resign(admin)",
			zap.Uint("id", uid), zap.String("gid", c.Param("gid")))
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.NoDataResponse(c, func() error {
		return g.AdminResign(uid, uint(gid))
	})
}

func Leave(c *gin.Context) {
	uid := ginx.GetUserID(c)
	gid, err := strconv.ParseUint(c.Param("gid"), 10, 64)
	if err != nil {
		logger.Warn("Failed to leave group: invaild param",
			zap.Uint("id", uid),
			zap.String("gid", c.Param("gid")))
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.NoDataResponse(c, func() error {
		return g.Leave(uid, uint(gid))
	})
}

func Kick(c *gin.Context) {
	from := ginx.GetUserID(c)
	to, err1 := strconv.ParseUint(c.Param("id"), 10, 64)
	gid, err2 := strconv.ParseUint(c.Param("gid"), 10, 64)
	if err1 != nil || err2 != nil {
		logger.Warn("Failed to kick member: invaild param",
			zap.Uint("from", from),
			zap.String("to", c.Param("id")),
			zap.String("gid", c.Param("gid")))
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.NoDataResponse(c, func() error {
		return g.Kick(from, uint(to), uint(gid))
	})
}

func AcceptInvite(c *gin.Context) {
	uid := ginx.GetUserID(c)
	var msg websocket.ChatMsg
	c.ShouldBindJSON(&msg)
	if msg.To != uid {
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.NoDataResponse(c, func() error {
		return g.AcceptInvite(msg)
	})
}

func RejectApply(c *gin.Context) {
	from := ginx.GetUserID(c)
	to, err1 := strconv.ParseUint(c.Param("id"), 10, 64)
	gid, err2 := strconv.ParseUint(c.Param("gid"), 10, 64)
	if err1 != nil || err2 != nil {
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.NoDataResponse(c, func() error {
		return g.RejectApply(from, uint(to), uint(gid))
	})
}

func AcceptApply(c *gin.Context) {
	from := ginx.GetUserID(c)
	to, err1 := strconv.ParseUint(c.Param("id"), 10, 64)
	gid, err2 := strconv.ParseUint(c.Param("gid"), 10, 64)
	if err1 != nil || err2 != nil {
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.NoDataResponse(c, func() error {
		return g.AcceptApply(from, uint(to), uint(gid))
	})
}

func ReleaseAnnounce(c *gin.Context) {
	var data *model.GroupAnnouncement
	uid := ginx.GetUserID(c)
	c.ShouldBindJSON(&data)
	data.CreatedAt = 0
	data.CreatedBy = uid
	ginx.NoDataResponse(c, func() error {
		return g.ReleaseAnnounce(data)
	})
}

func ViewAnnounce(c *gin.Context) {
	uid := ginx.GetUserID(c)
	gid, err := strconv.ParseUint(c.Param("gid"), 10, 64)
	var cursor *model.Cursor
	c.ShouldBindJSON(&cursor)

	if err != nil {
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.HasDataResponse(c, func() (any, error) {
		return g.ViewAnnounce(uid, uint(gid), cursor)
	})
}

func ViewLatestAnnounce(c *gin.Context) {
	uid := ginx.GetUserID(c)
	gid, err := strconv.ParseUint(c.Param("gid"), 10, 64)
	if err != nil {
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.HasDataResponse(c, func() (any, error) {
		return g.ViewLatestAnnounce(uid, uint(gid))
	})
}

func DeleteAnnounce(c *gin.Context) {
	uid := ginx.GetUserID(c)
	gid, err1 := strconv.ParseUint(c.Param("gid"), 10, 64)
	announceID, err2 := strconv.ParseUint(c.Param("id"), 10, 64)
	if err1 != nil || err2 != nil {
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.NoDataResponse(c, func() error {
		return g.DeleteAnnounce(uid, uint(gid), uint(announceID))
	})
}

func GroupList(c *gin.Context) {
	uid := ginx.GetUserID(c)
	ginx.HasDataResponse(c, func() (any, error) {
		return g.List(uid)
	})
}
