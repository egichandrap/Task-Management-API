package middleware

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// fakeStore implements IdempotencyStore in memory for unit tests.
type fakeStore struct {
	mu        sync.Mutex
	records   map[string]IdempotencyRecord
	getErr    error
	saveErr   error
	saveCalls int
}

func newFakeStore() *fakeStore {
	return &fakeStore{records: map[string]IdempotencyRecord{}}
}

func (f *fakeStore) Get(_ context.Context, key string) (*IdempotencyRecord, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, false, f.getErr
	}
	rec, ok := f.records[key]
	if !ok {
		return nil, false, nil
	}
	return &rec, true, nil
}

func (f *fakeStore) Save(_ context.Context, key string, status int, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	f.records[key] = IdempotencyRecord{Key: key, Status: status, Body: body, CreatedAt: time.Now()}
	f.saveCalls++
	return nil
}

// newIdempotencyRouter builds a gin engine with an authenticated user in the
// context and a single POST /tasks route guarded by the middleware.
func newIdempotencyRouter(store IdempotencyStore, userID string, handlerCalls *atomic.Int32, work time.Duration) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	r.POST("/tasks", Idempotency(store), func(c *gin.Context) {
		handlerCalls.Add(1)
		if work > 0 {
			time.Sleep(work)
		}
		c.JSON(http.StatusCreated, gin.H{"user": c.GetString("user_id")})
	})
	return r
}

func doPost(r *gin.Engine, key string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(`{}`))
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestIdempotency(t *testing.T) {
	t.Run("without key the handler runs and nothing is stored", func(t *testing.T) {
		store := newFakeStore()
		var calls atomic.Int32
		r := newIdempotencyRouter(store, "userA", &calls, 0)

		w := doPost(r, "")

		if w.Code != http.StatusCreated || calls.Load() != 1 {
			t.Fatalf("expected handler to run once with 201, got %d/%d", w.Code, calls.Load())
		}
		if len(store.records) != 0 || store.saveCalls != 0 {
			t.Fatal("expected nothing to be stored")
		}
	})

	t.Run("first request executes the handler and stores the original status", func(t *testing.T) {
		store := newFakeStore()
		var calls atomic.Int32
		r := newIdempotencyRouter(store, "userA", &calls, 0)

		w := doPost(r, "key-1")

		if w.Code != http.StatusCreated || calls.Load() != 1 {
			t.Fatalf("expected 201 with one handler call, got %d/%d", w.Code, calls.Load())
		}
		if store.saveCalls != 1 {
			t.Fatalf("expected one save, got %d", store.saveCalls)
		}
		if store.records["userA:key-1"].Status != http.StatusCreated {
			t.Fatal("expected the original 201 status code to be stored")
		}
	})

	t.Run("replay returns the stored response and status without running the handler", func(t *testing.T) {
		store := newFakeStore()
		var calls atomic.Int32
		r := newIdempotencyRouter(store, "userA", &calls, 0)

		doPost(r, "key-1")
		w := doPost(r, "key-1")

		if calls.Load() != 1 {
			t.Fatalf("expected handler to run only once, ran %d times", calls.Load())
		}
		if w.Code != http.StatusCreated {
			t.Fatalf("expected replayed status 201, got %d", w.Code)
		}
		if w.Body.String() != store.records["userA:key-1"].Body {
			t.Fatal("expected replayed body to match stored body")
		}
	})

	t.Run("the same key from another user never replays another user's response", func(t *testing.T) {
		store := newFakeStore()
		var callsA, callsB atomic.Int32
		rA := newIdempotencyRouter(store, "userA", &callsA, 0)
		rB := newIdempotencyRouter(store, "userB", &callsB, 0)

		wA := doPost(rA, "shared-key")
		wB := doPost(rB, "shared-key")

		if callsA.Load() != 1 || callsB.Load() != 1 {
			t.Fatalf("expected each user to execute the handler once, got %d and %d", callsA.Load(), callsB.Load())
		}
		if wB.Body.String() == wA.Body.String() {
			t.Fatal("expected userB to receive its own response, not userA's")
		}
		if _, ok := store.records["userB:shared-key"]; !ok {
			t.Fatal("expected a separate record scoped to userB")
		}
	})

	t.Run("store failure degrades to normal execution", func(t *testing.T) {
		store := newFakeStore()
		store.getErr = errors.New("db down")
		var calls atomic.Int32
		r := newIdempotencyRouter(store, "userA", &calls, 0)

		w := doPost(r, "key-1")

		if w.Code != http.StatusCreated || calls.Load() != 1 {
			t.Fatalf("expected handler to execute despite store failure, got %d/%d", w.Code, calls.Load())
		}
	})

	t.Run("error responses are not stored", func(t *testing.T) {
		store := newFakeStore()
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("user_id", "userA")
			c.Next()
		})
		r.POST("/broken", Idempotency(store), func(c *gin.Context) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad"})
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/broken", bytes.NewBufferString(`{}`))
		req.Header.Set("Idempotency-Key", "err-key")
		r.ServeHTTP(w, req)

		if _, ok := store.records["userA:err-key"]; ok {
			t.Fatal("expected error response not to be stored")
		}
	})

	t.Run("concurrent requests with the same key execute the handler once", func(t *testing.T) {
		store := newFakeStore()
		var calls atomic.Int32
		r := newIdempotencyRouter(store, "userA", &calls, 20*time.Millisecond)

		const n = 50
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				doPost(r, "race-key")
			}()
		}
		wg.Wait()

		if calls.Load() != 1 {
			t.Fatalf("expected handler to execute exactly once, ran %d times", calls.Load())
		}
		if store.saveCalls != 1 {
			t.Fatalf("expected exactly one save, got %d", store.saveCalls)
		}
	})

	t.Run("locks are cleaned up after the request", func(t *testing.T) {
		store := newFakeStore()
		var calls atomic.Int32
		r := newIdempotencyRouter(store, "userA", &calls, 0)

		doPost(r, "cleanup-key")

		if _, loaded := idempotencyLocks.Load("userA:cleanup-key"); loaded {
			t.Fatal("expected lock entry to be removed after the request")
		}
	})
}
