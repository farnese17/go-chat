package middleware

import (
	"slices"

	"github.com/farnese17/chat/registry"
	"github.com/farnese17/chat/utils/errorsx"
	"github.com/farnese17/chat/utils/ginx"
	"github.com/gin-gonic/gin"
)

func CheckManagerPermissions(permissions ...uint) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler := c.MustGet("from").(uint)
		u, err := registry.GetService().Manager().Get(handler)
		if err != nil {
			ginx.HandleError(c, err)
			return
		}

		if !slices.Contains(permissions, u.Permissions) {
			ginx.HandleError(c, errorsx.ErrPermissiondenied)
			return
		}
		c.Next()
	}
}
