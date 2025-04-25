package v1

import (
	"fmt"
	"net/http"

	"github.com/farnese17/chat/config"
	"github.com/farnese17/chat/middleware"
	"github.com/farnese17/chat/pkg/storage"
	"github.com/farnese17/chat/registry"
	"github.com/farnese17/chat/service"
	"github.com/farnese17/chat/service/model"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/farnese17/chat/utils/ginx"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var u *service.UserService

var fs storage.Storage

func SetupUserService(s registry.Service) {
	u = service.NewUserService(s)
	fs = s.Storage()
}

func Register(c *gin.Context) {
	var user model.User
	c.ShouldBindJSON(&user)
	user.ID = 0
	user.BanLevel = model.BanLevelNone
	user.BanExpireAt = 0
	user.CreatedAt = 0
	user.UpdatedAt = 0
	user.DeletedAt = gorm.DeletedAt{}
	ginx.NoDataResponse(c, func() error {
		return u.Register(&user)
	})
}

// 接受uid/手机号/邮箱
func SearchUser(c *gin.Context) {
	account := c.Query("account")
	ginx.HasDataResponse(c, func() (any, error) {
		return u.SreachUser(account)
	})
}

// 获取用户信息
func Get(c *gin.Context) {
	id := ginx.GetUserID(c)
	ginx.HasDataResponse(c, func() (any, error) {
		return u.Get(id)
	})
}

func DeleteUser(c *gin.Context) {
	id := ginx.GetUserID(c)
	ginx.NoDataResponse(c, func() error {
		return u.Delete(id)
	})
}

func UpdateUserInfo(c *gin.Context) {
	params := make(map[string]string)
	c.ShouldBindJSON(&params)
	value, field := params["value"], params["field"]
	id := ginx.GetUserID(c)
	ginx.NoDataResponse(c, func() error {
		return u.UpdateInfo(id, value, field)
	})
}

func UpdatePassword(c *gin.Context) {
	id := ginx.GetUserID(c)
	password := make(map[string]string)
	c.ShouldBindJSON(&password)
	ginx.NoDataResponse(c, func() error {
		return u.UpdatePassword(id, password)
	})
}

func Login(c *gin.Context) {
	var params map[string]string
	c.ShouldBindJSON(&params)
	user, err := u.Login(params["account"], params["password"])
	if err != nil {
		ginx.ResponseJson(c, err, nil)
		c.Abort()
		return
	}

	token, err := middleware.GenerateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  errorsx.GetStatusCode(errorsx.ErrInvalidToken),
			"message": errorsx.ErrInvalidToken.Error(),
		})
		c.Abort()
		return
	}
	// 插入、替换token
	expire := config.GetConfig().Common().TokenValidPeriod()
	registry.GetService().Cache().SetToken(user.ID, token, expire)

	c.JSON(http.StatusOK, gin.H{
		"status":  errorsx.GetStatusCode(errorsx.ErrNil),
		"message": errorsx.ErrNil.Error(),
		"token":   token,
		"data":    user,
	})
}

func LogOut(c *gin.Context) {
	u.Logout(ginx.GetUserID(c))
	ginx.ResponseJson(c, errorsx.ErrNil, nil)
}

func Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil || file == nil {
		ginx.HandleInvalidParam(c)
		return
	}
	filename := header.Filename
	id := ginx.GetUserID(c)
	ginx.HasDataResponse(c, func() (any, error) {
		return fs.Upload(id, file, filename)
	})
}

func Download(c *gin.Context) {
	handleGetFile(c, "attachment")
}

func GetFile(c *gin.Context) {
	handleGetFile(c, "inline")
}

func handleGetFile(c *gin.Context, disposition string) {
	id := c.Param("id")
	f, err := fs.Download(id)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"status":  http.StatusNotFound,
			"message": errorsx.ErrNotFound.Error()})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("%s; filename=%s;filename*=UTF-8''%s",
		disposition, f.Name, f.Name))
	c.File(f.Path)
}

func DeleteFile(c *gin.Context) {
	fileID := c.Param("id")
	id := ginx.GetUserID(c)
	ginx.NoDataResponse(c, func() error {
		return fs.Delete(id, fileID)
	})
}
