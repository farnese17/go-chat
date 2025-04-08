package middleware

import (
	"net/http"
	"strconv"

	"github.com/farnese17/chat/registry"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/farnese17/chat/utils/validator"
	"github.com/gin-gonic/gin"
)

func VerifyID() gin.HandlerFunc {
	return func(c *gin.Context) {
		from := c.MustGet("from").(uint) // from token,should be valid
		// if parse error,target will be 0,which will be rejected by ValidateUID
		to, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		if err := validator.ValidateUID(uint(to)); err != nil || from == uint(to) {
			c.JSON(http.StatusOK, gin.H{
				"status":  errorsx.GetStatusCode(errorsx.ErrInvalidParams),
				"message": errorsx.ErrInvalidParams.Error(),
			})
			c.Abort()
			return
		}
		c.Set("to", uint(to))
		c.Next()
	}
}

func VerifyAminID() gin.HandlerFunc {
	return func(c *gin.Context) {
		from := c.MustGet("from").(uint) // from token,should be valid
		// if parse error,target will be 0,which will be rejected by ValidateUID
		to, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		if to < 1001 || to > 99999 || from == uint(to) {
			c.JSON(http.StatusOK, gin.H{
				"status":  errorsx.GetStatusCode(errorsx.ErrInvalidParams),
				"message": errorsx.ErrInvalidParams.Error(),
			})
			c.Abort()
			return
		}
		c.Set("to", uint(to))
		c.Next()
	}
}

func BanFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		s := registry.GetService()
		from := c.MustGet("from").(uint)
		to, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		if err := validator.ValidateUID(uint(to)); err != nil || from == uint(to) {
			c.JSON(http.StatusOK, gin.H{
				"status":  errorsx.GetStatusCode(errorsx.ErrInvalidParams),
				"message": errorsx.ErrInvalidParams.Error(),
			})
			c.Abort()
			return
		}
		if s.Cache().BFM().IsBanned(uint(to)) && s.Cache().IsBanned(uint(to)) {
			c.JSON(http.StatusOK, gin.H{
				"status":  errorsx.GetStatusCode(errorsx.ErrUserBanned),
				"message": errorsx.ErrUserBanned.Error(),
			})
			c.Abort()
			return
		}
		c.Set("to", uint(to))
		c.Next()
	}
}
