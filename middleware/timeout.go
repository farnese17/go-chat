package middleware

import (
    "context"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
)

func Timeout() gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
        defer cancel()
        c.Request = c.Request.WithContext(ctx)

        go func() {
            c.Next()
        }()

        done := make(chan struct{})
        select {
        case <-ctx.Done():
            c.JSON(http.StatusGatewayTimeout, gin.H{"message": "请求超时"})
            return
        case <-done:
            return
        }
    }
}

