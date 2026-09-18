package middleware

import (
	"task-api/internal/domain"
	"bytes"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
	"sync"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// In-memory mutex for concurrency lock to prevent race condition during idempotency check
var idempotencyLocks = sync.Map{}

func Idempotency(repo domain.TaskRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		idemKey := c.GetHeader("Idempotency-Key")
		if idemKey == "" {
			c.Next()
			return
		}

		// Use sync.Map to get a lock for this specific idempotency key
		lockInterface, _ := idempotencyLocks.LoadOrStore(idemKey, &sync.Mutex{})
		lock := lockInterface.(*sync.Mutex)
		
		lock.Lock()
		defer lock.Unlock()
		
		record, err := repo.GetIdempotency(c.Request.Context(), idemKey)
		if err == nil && record != nil {
			if time.Since(record.CreatedAt) < 24*time.Hour {
				c.Header("Content-Type", "application/json")
				c.String(http.StatusOK, record.Response) // Or created status based on original
				c.Abort()
				return
			}
		}

		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		c.Next()

		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			_ = repo.SaveIdempotency(c.Request.Context(), idemKey, blw.body.String())
		}
	}
}
