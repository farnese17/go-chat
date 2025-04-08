package ginx

import (
	"net/http"
	"strconv"

	"github.com/farnese17/chat/utils/errorsx"
	"github.com/gin-gonic/gin"
)

const userIDKey = "from"

func GetUserID(c *gin.Context) uint {
	return c.MustGet(userIDKey).(uint)
}

func ManagerGetID(c *gin.Context) uint {
	p := c.Param("id")
	id, err := strconv.Atoi(p)
	if err != nil {
		HandleInvalidParam(c)
		return 0
	}
	return uint(id)
}

func NoDataResponse(c *gin.Context, fn func() error) {
	if err := fn(); err != nil {
		HandleError(c, err)
		return
	}
	ResponseJson(c, errorsx.ErrNil, nil)
}

func HasDataResponse(c *gin.Context, fn func() (any, error)) {
	data, err := fn()
	if err != nil {
		HandleError(c, err)
		return
	}
	ResponseJson(c, errorsx.ErrNil, data)
}

func ResponseJson(c *gin.Context, err error, data interface{}) {
	resp := gin.H{
		"status":  errorsx.GetStatusCode(err),
		"message": err.Error(),
	}
	if data != nil {
		resp["data"] = data
	}
	c.JSON(http.StatusOK, resp)
}

func HandleError(c *gin.Context, err error) {
	ResponseJson(c, err, nil)
	c.Abort()
}

func HandleInvalidParam(c *gin.Context) {
	ResponseJson(c, errorsx.ErrInvalidParams, nil)
	c.Abort()
}
