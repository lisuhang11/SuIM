package webhook

import (
	"errors"
	"log/slog"
	"sync"
)

const (
	webhookWorkerCount = 2
	webhookBufferSize  = 100

	// Key is the query parameter name callers use to pass a webhook key.
	Key = "key"
)

var (
	ErrQueueFull    = errors.New("webhook queue is full")
	ErrQueueStopped = errors.New("webhook queue is stopped")
)

// MemoryQueue runs callback posts on a fixed set of workers.
type MemoryQueue struct {
	mu      sync.Mutex
	ch      chan func()
	stopped bool
}

func NewMemoryQueue(workerCount, bufferSize int) *MemoryQueue {
	if workerCount <= 0 {
		workerCount = webhookWorkerCount
	}
	if bufferSize <= 0 {
		bufferSize = webhookBufferSize
	}
	q := &MemoryQueue{ch: make(chan func(), bufferSize)}
	for i := 0; i < workerCount; i++ {
		go q.worker()
	}
	return q
}

// Push enqueues fn. It returns ErrQueueFull when the buffer is full and ErrQueueStopped after Stop.
func (q *MemoryQueue) Push(fn func()) error {
	if fn == nil {
		return errors.New("nil webhook task")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.stopped {
		return ErrQueueStopped
	}
	select {
	case q.ch <- fn:
		return nil
	default:
		return ErrQueueFull
	}
}

// Stop rejects later pushes and lets workers drain tasks already in the buffer.
func (q *MemoryQueue) Stop() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.stopped {
		return
	}
	q.stopped = true
	close(q.ch)
}

func (q *MemoryQueue) worker() {
	for fn := range q.ch {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("webhook queue task panicked", "panic", r)
				}
			}()
			fn()
		}()
	}
}
