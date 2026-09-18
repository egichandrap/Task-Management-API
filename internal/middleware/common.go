package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"log/slog"
	"task-api/pkg/errors"
	"task-api/pkg/logger"
	"task-api/pkg/response"
	"time"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		reqID := uuid.New().String()
		c.Set("request_id", reqID)

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		logArgs := []any{
			slog.String("request_id", reqID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status_code", status),
			slog.Duration("latency", latency),
		}

		if status >= 500 {
			logger.Log.Error("Server Error", logArgs...)
		} else if status >= 400 {
			logger.Log.Warn("Client Error", logArgs...)
		} else {
			logger.Log.Info("Request Processed", logArgs...)
		}
	}
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Log.Error("Panic Recovered", slog.Any("error", err))
				response.JSONError(c, 500, "INTERNAL_SERVER_ERROR", "An unexpected error occurred")
			}
		}()

		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			if appErr, ok := err.(*customerrors.AppError); ok {
				response.JSONError(c, appErr.Status, appErr.Code, appErr.Message)
				return
			}
			// Unexpected (possibly infrastructure) error: log the details,
			// but never expose them to the API consumer.
			logger.Log.Error("Unhandled error", slog.Any("error", err), slog.String("request_id", c.GetString("request_id")))
			response.JSONError(c, 500, "INTERNAL_SERVER_ERROR", "An unexpected error occurred")
		}
	}
}
