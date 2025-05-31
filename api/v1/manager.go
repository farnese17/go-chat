package v1

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/farnese17/chat/middleware"
	"github.com/farnese17/chat/registry"
	"github.com/farnese17/chat/service"
	"github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/farnese17/chat/utils/ginx"
	"github.com/farnese17/chat/websocket"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	banFailed   = "Filed to ban user"
	banned      = "Banned user"
	unbanFailed = "Failed to unban user"
	unban       = "Unban user"
)

var mgr *service.Manager

func SetupManagerService(s registry.Service) {
	mgr = service.NewManager(s)
}

func logOperation(err error, message string, handler uint, userID string, banLevel string, banExpire int) {
	if err != nil {
		registry.GetService().Logger().Error(message,
			zap.Error(err),
			zap.Uint("handler", handler),
			zap.String("user_id", userID),
			zap.String("ban_level", banLevel),
			zap.Int("ban_days", banExpire))
	} else {
		registry.GetService().Logger().Info(message,
			zap.Uint("handler", handler),
			zap.String("user_id", userID),
			zap.String("ban_level", banLevel),
			zap.Int("ban_days", banExpire))
	}
}

func StopWebsocket(c *gin.Context) {
	s := registry.GetService()
	if s.Hub() == nil {
		ginx.HandleError(c, errorsx.ErrServerClosed)
		return
	}

	ginx.NoDataResponse(c, func() error {
		err := websocket.StopWebsocket(s)
		if err != nil {
			s.Logger().Error("Failed to stop websocket goroutine", zap.Error(err), zap.Uint("handler", ginx.GetUserID(c)))
		} else {
			s.Logger().Info("Stop websocket goroutine", zap.Uint("handler", ginx.GetUserID(c)))
		}
		return err
	})
}

func StartWebsocket(c *gin.Context) {
	service := registry.GetService()
	if service.Hub() != nil {
		ginx.ResponseJson(c, errorsx.ErrServerStarted, nil)
		c.Abort()
		return
	}
	service.SetHub(websocket.NewHubInterface(service))
	service.Logger().Info("Start websocket goroutine", zap.Uint("handler", ginx.GetUserID(c)))
	ginx.ResponseJson(c, errorsx.ErrNil, nil)
}

func GetConfig(c *gin.Context) {
	s := registry.GetService()
	cfg := s.Config().Get()
	ginx.HasDataResponse(c, func() (any, error) {
		return cfg, nil
	})
}

func SetConfig(c *gin.Context) {
	body := struct {
		Section string `json:"section"`
		Key     string `json:"key"`
		Value   string `json:"value"`
	}{}

	if err := c.ShouldBindJSON(&body); err != nil {
		ginx.HandleError(c, err)
		return
	}
	if body.Value == "" {
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.NoDataResponse(c, func() error {
		s := registry.GetService()
		cfg := s.Config()
		var err error
		switch body.Section {
		case "common":
			err = cfg.SetCommon(body.Key, body.Value)
		case "cache":
			err = cfg.SetCache(body.Key, body.Value)
		default:
			err = errors.New("section: common,cache")
		}
		if err == nil {
			s.Logger().Info("Modify config",
				zap.Uint("handler", ginx.GetUserID(c)),
				zap.String("section", body.Section),
				zap.String("key", body.Key),
				zap.String("value", body.Value))
		}

		return err
	})
}

func SaveConfig(c *gin.Context) {
	s := registry.GetService()
	cfg := s.Config()
	ginx.NoDataResponse(c, func() error {
		s.Logger().Info("Save config", zap.Uint("handler", ginx.GetUserID(c)))
		return cfg.Save()
	})
}

func BanUserTemp(c *gin.Context) {
	id := c.Param("id")
	ginx.NoDataResponse(c, func() error {
		err := mgr.Ban(id, model.BanLevelTemporary, 7)
		if err != nil {
			logOperation(err, banFailed, ginx.GetUserID(c), id, "ban_temporary", 7)
			return err
		}
		logOperation(err, banned, ginx.GetUserID(c), id, "ban_temporary", 7)
		return nil
	})
}

func BanUserPerma(c *gin.Context) {
	id := c.Param("id")
	ginx.NoDataResponse(c, func() error {
		err := mgr.Ban(id, model.BanLevelPermanent, 100*365)
		if err != nil {
			logOperation(err, banFailed, ginx.GetUserID(c), id, "ban_permanent", 100*365)
			return err
		}
		logOperation(err, banned, ginx.GetUserID(c), id, "ban_permanent", 100*365)
		return nil
	})
}

func BanUserNoPost(c *gin.Context) {
	id := c.Param("id")
	ginx.NoDataResponse(c, func() error {
		err := mgr.Ban(id, model.BanLevelNoPost, 3)
		if err != nil {
			logOperation(err, banFailed, ginx.GetUserID(c), id, "no_post", 3)
			return err
		}
		logOperation(err, banned, ginx.GetUserID(c), id, "no_post", 3)
		return nil
	})
}

func BanUserMuted(c *gin.Context) {
	id := c.Param("id")
	ginx.NoDataResponse(c, func() error {
		err := mgr.Ban(id, model.BanLevelMuted, 1)
		if err != nil {
			logOperation(err, banFailed, ginx.GetUserID(c), id, "muted", 1)
			return err
		}
		logOperation(err, banned, ginx.GetUserID(c), id, "muted", 1)
		return nil
	})
}

func UnbanUser(c *gin.Context) {
	id := c.Param("id")
	ginx.NoDataResponse(c, func() error {
		err := mgr.Unban(id)
		if err != nil {
			logOperation(err, unbanFailed, ginx.GetUserID(c), id, "unban", 0)
			return err
		}
		logOperation(err, unban, ginx.GetUserID(c), id, "unban", 0)
		return nil
	})
}

func CreateAdmin(c *gin.Context) {
	handler := ginx.GetUserID(c)
	var data *model.Manager
	if err := c.ShouldBindJSON(&data); err != nil {
		ginx.HandleError(c, err)
		return
	}
	ginx.HasDataResponse(c, func() (any, error) {
		id, err := mgr.CreateAdmin(data)
		if err == nil {
			registry.GetService().Logger().Info("Created administrator",
				zap.Uint("handler", handler), zap.Uint("new_admin", id))
		}
		return id, err
	})
}

func DeleteAdmin(c *gin.Context) {
	handler := ginx.GetUserID(c)
	id := ginx.ManagerGetID(c)
	ginx.NoDataResponse(c, func() error {
		err := mgr.DeleteAdmin(id)
		if err != nil {
			registry.GetService().Logger().Error("Failed to delete admin",
				zap.Error(err), zap.Uint("handler", handler), zap.Uint("admin_id", id))
			return err
		}
		registry.GetService().Logger().Info("Deleted admin",
			zap.Uint("handler", handler), zap.Uint("admin_id", id))
		return nil
	})
}

func RestoreAdministrator(c *gin.Context) {
	handler := ginx.GetUserID(c)
	id := ginx.ManagerGetID(c)
	ginx.NoDataResponse(c, func() error {
		err := mgr.RestoreAdministrator(id)
		if err != nil {
			registry.GetService().Logger().Error("Failed to restore admin",
				zap.Error(err), zap.Uint("handler", handler), zap.Uint("admin_id", id))
			return err
		}
		registry.GetService().Logger().Info("Restored admin",
			zap.Uint("handler", handler), zap.Uint("admin_id", id))
		return nil
	})
}

func SetPermission(c *gin.Context) {
	handler := ginx.GetUserID(c)
	id := ginx.ManagerGetID(c)
	if handler == id {
		ginx.HandleError(c, errorsx.ErrPermissiondenied)
		return
	}
	permission, _ := strconv.Atoi(c.Query("permission"))
	permissions := []uint{model.MgrWriteAndRead, model.MgrSuperAdministrator, model.MgrOnlyRead}
	if !slices.Contains(permissions, uint(permission)) {
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.NoDataResponse(c, func() error {
		err := mgr.Setpermission(id, uint(permission))
		if err != nil {
			registry.GetService().Logger().Error("Failed to set permission",
				zap.Error(err), zap.Uint("handler", handler), zap.Uint("admin_id", id))
			return err
		}
		registry.GetService().Logger().Info("Set permission",
			zap.Uint("handler", handler), zap.Uint("admin_id", id))
		return nil
	})
}

func BannedUserList(c *gin.Context) {
	var cursor *model.Cursor
	if err := c.ShouldBindJSON(&cursor); err != nil {
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.HasDataResponse(c, func() (any, error) {
		return mgr.BannedUserList(cursor)
	})
}

func CountBannedUser(c *gin.Context) {
	ginx.HasDataResponse(c, func() (any, error) {
		return mgr.CountBannedUser()
	})
}

func AdminList(c *gin.Context) {
	var cursor *model.Cursor
	if err := c.ShouldBindJSON(&cursor); err != nil {
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.HasDataResponse(c, func() (any, error) {
		return mgr.AdminList(cursor)
	})
}

func GetAdmin(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.HasDataResponse(c, func() (any, error) {
		return mgr.GetAdmin(uint(id))
	})
}

func AdminUpdatePassword(c *gin.Context) {
	handler := ginx.GetUserID(c)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ginx.HandleInvalidParam(c)
		return
	}
	pw := make(map[string]string)
	if err := c.ShouldBindJSON(&pw); err != nil {
		ginx.HandleError(c, err)
		return
	}
	ginx.NoDataResponse(c, func() error {
		return mgr.UpdatePassword(handler, uint(id), pw["new"], pw["confirm"])
	})
}

func AdminLogin(c *gin.Context) {
	s := registry.GetService()
	var login map[string]any
	if err := c.ShouldBindJSON(&login); err != nil {
		ginx.HandleInvalidParam(c)
		return
	}

	id, ok1 := login["id"].(float64)
	passwd, ok2 := login["password"].(string)
	if !ok1 || !ok2 {
		s.Logger().Error("Failed to log in as administrator",
			zap.Any("id", login["id"]))
		ginx.HandleInvalidParam(c)
		return
	}
	ginx.HasDataResponse(c, func() (any, error) {
		err := mgr.Login(uint(id), passwd)
		if err != nil {
			s.Logger().Error("Failed to log in as administrator",
				zap.Error(err), zap.Float64("id", id))
			return nil, err
		}
		s.Logger().Info("Administrator log in", zap.Float64("id", id))
		token, err := middleware.GenerateToken(uint(id))
		if err != nil {
			return nil, err
		}
		s.Cache().SetToken(uint(id), token, s.Config().Common().TokenValidPeriod())
		return token, nil
	})
}

func Healthy(c *gin.Context) {
	s := registry.GetService()
	details, _ := strconv.ParseBool(c.Query("details"))

	importantStatus := importantHealthyStatus(s)
	if !details {
		c.JSON(http.StatusOK, importantStatus)
		return
	}

	detailStatus := detailHealthyStatus(s)
	c.JSON(http.StatusOK, detailStatus)
}

func importantHealthyStatus(s registry.Service) map[string]any {
	wsStatus := "up"
	dbStatus := "up"
	rcStatus := "up"
	total := 3
	downCount := 0
	if s.Hub() == nil || s.Hub().IsClosed() {
		wsStatus = "down"
		downCount++
	}
	if !s.Manager().Healthy() {
		dbStatus = "down"
		downCount++
	}
	if !s.Cache().Healthy() {
		rcStatus = "down"
		downCount++
	}

	status := "healthy"
	if downCount > 0 && downCount < total {
		status = "degraded"
	} else if downCount == total {
		status = "critical"
	}

	return map[string]any{
		"status": status,
		"uptime": s.Uptime(),
		"services": map[string]any{
			"websocket": wsStatus,
			"database":  dbStatus,
			"cache":     rcStatus,
		},
	}
}

func detailHealthyStatus(s registry.Service) map[string]any {
	status := importantHealthyStatus(s)

	var wsConnections int
	var wsUptime time.Duration
	if s.Hub() != nil {
		wsConnections = s.Hub().Count()
		wsUptime = s.Hub().Uptime()
	}

	res := map[string]any{
		"status": status["status"],
		"services": map[string]any{
			"websocket": map[string]any{
				"status":      status["services"].(map[string]any)["websocket"],
				"connections": wsConnections,
				"uptime":      wsUptime.Round(time.Second).String(),
			},
			"database": map[string]any{
				"status": status["services"].(map[string]any)["database"],
				"stats":  s.Manager().Stats(),
			},
			"cache": map[string]any{
				"status": status["services"].(map[string]any)["cache"],
				"stats":  s.Cache().Stats(),
			},
			"system": map[string]any{
				"cpu":    mgr.GetCPUState(),
				"memory": mgr.GetMemoryState(),
				"disk":   mgr.GetDiskState(),
				"uptime": s.Uptime().Round(time.Second).String(),
			},
		},
	}
	return res
}
