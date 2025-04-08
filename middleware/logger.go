package middleware

import (
	"time"

	"github.com/farnese17/chat/utils/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := logger.GetLogger()
		start := time.Now()
		c.Next()
		end := time.Now()
		statusCode := c.Writer.Status()
		requestLogger := logger.With(
			zap.Time("Time", start),
			zap.String("ClientIP", c.ClientIP()),
			zap.String("Method", c.Request.Method),
			zap.String("Path", c.Request.URL.Path),
			zap.String("Protocol", c.Request.Proto),
			zap.Int("Status", statusCode),
			zap.Duration("Spendtime", end.Sub(start)),
			zap.String("Agent", c.Request.UserAgent()),
		)
		if statusCode >= 500 {
			err := c.Errors.String()
			requestLogger.Error("HandleRequest", zap.String("error", err))
		} else if statusCode >= 400 {
			requestLogger.Warn("HandleRequest")
		} else {
			requestLogger.Info("HandleRequest")
		}
	}
}
