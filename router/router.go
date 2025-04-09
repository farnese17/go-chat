package router

import (
	"net/http"

	v1 "github.com/farnese17/chat/api/v1"
	"github.com/farnese17/chat/middleware"
	"github.com/farnese17/chat/service/model"
	"github.com/gin-gonic/gin"
)

func SetupRouter(mode string) *gin.Engine {
	gin.SetMode(mode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.Cors())

	r.GET("/api/v1/files/download/:id", v1.Download)
	r.GET("/api/v1/files/:id", v1.GetFile)

	auth := r.Group("api/v1")
	auth.Use(middleware.JWT(http.StatusOK))
	auth.Use(middleware.VerifyTokenInWhitelist(http.StatusOK))
	{
		auth.POST("/logout", v1.LogOut)

		// files
		files := auth.Group("/files")
		files.POST("", v1.Upload)
		files.DELETE("/:id", v1.DeleteFile)

		users := auth.Group("/users")
		// user
		users.GET("", v1.Get)
		users.GET("/search", v1.SearchUser)
		users.DELETE("", v1.DeleteUser)
		users.PUT("", v1.UpdateUserInfo)
		users.PUT("/password", v1.UpdatePassword)

		// group
		groupCheckBan := auth.Group("/groups")
		groupCheckBan.Use(middleware.BanFilter())
		groupCheckBan.POST("/:gid/invitations/:id", v1.Invite)
		groupCheckBan.PUT("/:gid/applications/:id/accept", v1.AcceptApply)
		groupCheckBan.PUT("/:gid/owner/:id", v1.HandOverOwner)

		group := auth.Group("/groups")
		group.POST("", v1.Create)
		group.GET("", v1.GroupList)
		group.GET("/:gid", v1.SearchByID)
		group.GET("/search", v1.SearchByName)
		group.PUT("/:gid/invitations/accept", v1.AcceptInvite)
		group.POST("/:gid/applications", v1.Apply)
		group.PUT("/:gid/applications/:id/reject", v1.RejectApply)
		group.GET("/:gid/members/:id", v1.Member)
		group.GET("/:gid/members", v1.Members)
		group.DELETE("/:gid", v1.Delete)
		group.PUT("/:gid", v1.Update)
		group.PUT("/:gid/admins/:id", v1.ModifyAdmin)
		group.PUT("/:gid/admins/me/resign", v1.AdminResign)
		group.DELETE("/:gid/members/me", v1.Leave)
		group.DELETE("/:gid/members/:id", v1.Kick)

		group.POST("/:gid/announces", v1.ReleaseAnnounce)
		group.GET("/:gid/announces", v1.ViewAnnounce)
		group.GET("/:gid/announces/latest", v1.ViewLatestAnnounce)
		group.DELETE("/:gid/announces/:id", v1.DeleteAnnounce)

		// friend
		friendCheckBan := auth.Group("/friends")
		friendCheckBan.Use(middleware.BanFilter())
		friendCheckBan.GET("/:id", v1.GetFriend)
		friendCheckBan.POST("/request/:id", v1.Request)
		friendCheckBan.PUT("/accept/:id", v1.Accept)

		friendValidateIDOnly := auth.Group("/friends")
		friendValidateIDOnly.Use(middleware.VerifyID())
		friendValidateIDOnly.PUT("/reject/:id", v1.Reject)
		friendValidateIDOnly.DELETE("/:id", v1.RemoveFriend)
		friendValidateIDOnly.PUT("/block/:id", v1.BlockFriend)
		friendValidateIDOnly.PUT("/unblock/:id", v1.UnblockFriend)
		friendValidateIDOnly.PUT("/remark/:id", v1.SetRemark)
		friendValidateIDOnly.PUT("/setgroup/:id", v1.SetGroup)

		auth.GET("/friends/search", v1.SearchFriend)
		auth.GET("/friends", v1.FriendList)
	}
	public := r.Group("api/v1")
	{
		public.POST("/users", v1.Register)
		public.POST("/login", v1.Login)
	}

	// websocket
	ws := r.Group("/api/v1")
	ws.Use(middleware.JWT(http.StatusUnauthorized), middleware.VerifyTokenInWhitelist(http.StatusUnauthorized)).
		GET("/ws", v1.WsRoutes)

	return r
}

func SetupManagerRouter(mode string) *gin.Engine {
	gin.SetMode(mode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.Cors())

	r.POST("/api/v1/managers/login", v1.AdminLogin)

	auth := r.Group("api/v1/managers")
	auth.Use(middleware.JWT(http.StatusOK),
		middleware.VerifyTokenInWhitelist(http.StatusOK))

	hasReadPermissions := auth.Group("")
	hasReadPermissions.Use(middleware.CheckManagerPermissions(
		model.MgrWriteAndRead, model.MgrSuperAdministrator, model.MgrOnlyRead))
	{
		hasReadPermissions.GET("/config", v1.GetConfig)
		hasReadPermissions.GET("/users/banned", v1.BannedUserList)
		hasReadPermissions.GET("/users/banned/count", v1.CountBannedUser)
		hasReadPermissions.GET("/admins", v1.AdminList)
		hasReadPermissions.GET("/admins/:id", v1.GetAdmin)
		hasReadPermissions.PUT("/admins/:id/update/password", v1.AdminUpdatePassword)
	}

	hasWritePermissions := auth.Group("")
	hasWritePermissions.Use(middleware.CheckManagerPermissions(
		model.MgrWriteAndRead, model.MgrSuperAdministrator))
	{
		hasWritePermissions.POST("/ws/start", v1.StartWebsocket)
		hasWritePermissions.POST("/ws/stop", v1.StopWebsocket)
		hasWritePermissions.PUT("/config/set", v1.SetConfig)
		hasWritePermissions.PUT("/config/save", v1.SaveConfig)
		hasWritePermissions.PUT("/users/:id/ban/temp", v1.BanUserTemp)
		hasWritePermissions.PUT("/users/:id/ban/perma", v1.BanUserPerma)
		hasWritePermissions.PUT("/users/:id/ban/nopost", v1.BanUserNoPost)
		hasWritePermissions.PUT("/users/:id/ban/mute", v1.BanUserMuted)
		hasWritePermissions.PUT("/users/:id/ban/unban", v1.UnbanUser)
	}

	hasSuperPermission := auth.Group("")
	hasSuperPermission.Use(middleware.CheckManagerPermissions(model.MgrSuperAdministrator))
	{
		hasSuperPermission.POST("", v1.CreateAdmin)
		hasSuperPermission.DELETE("/admins/:id", v1.DeleteAdmin)
		hasSuperPermission.PUT("/admins/:id/restore", v1.RestoreAdministrator)
		hasSuperPermission.PUT("/admins/:id/permissions", v1.SetPermission)

	}
	return r
}
