// Package httpx holds HTTP helpers shared by the API handlers.
package httpx

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Abort answers with a machine-readable error and stops the handler chain.
func Abort(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

// Internal records err for the request log and answers 500 without details,
// so database or broker errors never reach the client.
func Internal(c *gin.Context, err error) {
	_ = c.Error(err)
	Abort(c, http.StatusInternalServerError, "internal_error", "internal error")
}

// RequestLogger logs every request and the errors recorded by handlers.
func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		fields := []zap.Field{
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Duration("latency", time.Since(start)),
		}
		if len(c.Errors) > 0 {
			logger.Error("request failed", append(fields, zap.String("errors", c.Errors.String()))...)
			return
		}
		logger.Info("request", fields...)
	}
}
