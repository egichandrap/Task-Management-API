package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"task-api/internal/domain"
	"github.com/gin-gonic/gin"
)

type mockIdempotencyRepo struct {
	mu      sync.Mutex
	records map[string]*domain.IdempotencyRecord
	calls   int
}

func newMockRepo() *mockIdempotencyRepo {
	return &mockIdempotencyRepo{
		records: make(map[string]*domain.IdempotencyRecord),
	}
}

func (m *mockIdempotencyRepo) SaveIdempotency(ctx context.Context, key string, response string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records[key] = &domain.IdempotencyRecord{
		Key:       key,
		Response:  response,
		CreatedAt: time.Now(),
	}
	m.calls++
	return nil
}

func (m *mockIdempotencyRepo) GetIdempotency(ctx context.Context, key string) (*domain.IdempotencyRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if record, exists := m.records[key]; exists {
		return record, nil
	}
	return nil, nil // Not found
}

// Dummy methods to satisfy interface
func (m *mockIdempotencyRepo) Create(ctx context.Context, task *domain.Task) error { return nil }
func (m *mockIdempotencyRepo) FindAll(ctx context.Context, filter domain.TaskFilter) ([]domain.Task, int64, error) { return nil, 0, nil }
func (m *mockIdempotencyRepo) FindByID(ctx context.Context, id string) (*domain.Task, error) { return nil, nil }
func (m *mockIdempotencyRepo) Update(ctx context.Context, task *domain.Task) error { return nil }
func (m *mockIdempotencyRepo) Delete(ctx context.Context, id string) error { return nil }
func (m *mockIdempotencyRepo) AssignTaskTx(ctx context.Context, taskID, newAssigneeID, changedBy string) error { return nil }


func TestIdempotency_RaceCondition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMockRepo()
	router := gin.New()
	
	// Reset locks
	idempotencyLocks = sync.Map{}

	router.Use(Idempotency(repo))
	
	var handlerCalls int
	var handlerMu sync.Mutex
	
	router.POST("/tasks", func(c *gin.Context) {
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		
		handlerMu.Lock()
		handlerCalls++
		handlerMu.Unlock()
		
		c.JSON(http.StatusCreated, gin.H{"message": "Task created"})
	})

	t.Run("Sequential Requests", func(t *testing.T) {
		req1, _ := http.NewRequest("POST", "/tasks", nil)
		req1.Header.Set("Idempotency-Key", "seq-key-1")
		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, req1)
		
		if w1.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", w1.Code)
		}

		req2, _ := http.NewRequest("POST", "/tasks", nil)
		req2.Header.Set("Idempotency-Key", "seq-key-1")
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK && w2.Code != http.StatusCreated {
			t.Errorf("expected status 200 or 201, got %d", w2.Code)
		}
		
		if repo.calls != 1 {
			t.Errorf("expected repo save calls to be 1, got %d", repo.calls)
		}
	})

	t.Run("Concurrent Requests", func(t *testing.T) {
		handlerCalls = 0 // reset
		repo.calls = 0   // reset repo save calls
		
		concurrentCount := 50
		var wg sync.WaitGroup
		wg.Add(concurrentCount)

		for i := 0; i < concurrentCount; i++ {
			go func() {
				defer wg.Done()
				req, _ := http.NewRequest("POST", "/tasks", nil)
				req.Header.Set("Idempotency-Key", "concurrent-key-1")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}()
		}
		wg.Wait()

		if handlerCalls != 1 {
			t.Errorf("expected handler to be called exactly once, got %d", handlerCalls)
		}
		if repo.calls != 1 {
			t.Errorf("expected repo save to be called exactly once, got %d", repo.calls)
		}
	})
}
