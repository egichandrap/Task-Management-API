package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// IdempotencyRecord is a stored response that can be replayed.
type IdempotencyRecord struct {
	Key       string
	Status    int
	Body      string
	CreatedAt time.Time
}

// IdempotencyStore is the narrow port this middleware needs.
// It is defined here (consumer side) so the transport layer depends on a
// small abstraction instead of a repository or persistence detail.
type IdempotencyStore interface {
	// Get returns the stored record for key, whether it was found, and any
	// infrastructure error. Expired entries are reported as not found.
	Get(ctx context.Context, key string) (*IdempotencyRecord, bool, error)
	Save(ctx context.Context, key string, status int, body string) error
}

// Per-key mutexes guarding the check-then-execute window of the handler.
// Entries are removed on release so the map cannot grow unboundedly.
var idempotencyLocks = sync.Map{}

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// Idempotency makes requests safe to retry when the client supplies an
// Idempotency-Key header. Keys are scoped per user so one user can never
// replay another user's response.
func Idempotency(store IdempotencyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		idemKey := c.GetHeader("Idempotency-Key")
		if idemKey == "" {
			c.Next()
			return
		}

		key := c.GetString("user_id") + ":" + idemKey

		lockEntry, _ := idempotencyLocks.LoadOrStore(key, &sync.Mutex{})
		lock := lockEntry.(*sync.Mutex)
		lock.Lock()
		defer func() {
			lock.Unlock()
			// Remove the lock entry if it is still ours, so the map
			// does not accumulate one mutex per key forever.
			if current, ok := idempotencyLocks.Load(key); ok && current == lockEntry {
				idempotencyLocks.Delete(key)
			}
		}()

		record, found, err := store.Get(c.Request.Context(), key)
		if err != nil {
			// Store failure must not block the request; it only loses replay protection.
			slog.Default().Warn("idempotency lookup failed", slog.Any("error", err))
		} else if found {
			c.Header("Content-Type", "application/json")
			c.String(record.Status, record.Body)
			c.Abort()
			return
		}

		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		c.Next()

		if s := c.Writer.Status(); s >= 200 && s < 300 {
			if err := store.Save(c.Request.Context(), key, s, blw.body.String()); err != nil {
				slog.Default().Warn("idempotency save failed", slog.Any("error", err))
			}
		}
	}
}
