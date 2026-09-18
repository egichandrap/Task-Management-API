package response

import (
	"github.com/gin-gonic/gin"
	"time"
)

type ErrorResponse struct {
	Status    string    `json:"status"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type SuccessResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Meta   interface{} `json:"meta,omitempty"`
}

func JSONError(c *gin.Context, statusCode int, code, message string) {
	c.AbortWithStatusJSON(statusCode, ErrorResponse{
		Status:    "error",
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
	})
}

func JSONSuccess(c *gin.Context, statusCode int, data interface{}, meta interface{}) {
	c.JSON(statusCode, SuccessResponse{
		Status: "success",
		Data:   data,
		Meta:   meta,
	})
}
