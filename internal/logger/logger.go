package logger

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var Logger *zap.Logger

func GetLogger() *zap.Logger {
	if Logger == nil {
		var err error
		Logger, err = InitLogger()
		if err != nil {
			panic(err)
		}
	}
	return Logger
}

func InitLogger() (*zap.Logger, error) {
	return zap.NewProduction()
}

func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method
		raw := c.Request.URL.RawQuery

		c.Next()

		duration := time.Since(start)
		clientIP := c.ClientIP()
		statusCode := c.Writer.Status()
		requestHeader := c.Request.Header
		logger.Info("request",
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.String("method", method),
			zap.Reflect("header", requestHeader),
			zap.String("ip", clientIP),
			zap.Duration("duration", duration),
			zap.String("query", raw),
		)
	}
}
