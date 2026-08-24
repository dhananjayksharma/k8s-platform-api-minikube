package queue

import (
	"errors"
	"sync"
	"sync/atomic"
)

type Job struct {
	ID       int64
	Name     string
	Priority int
}

type WorkQueue struct {
	ch        chan Job
	workers   int
	handler   func(Job)
	stopOnce  sync.Once
	wg        sync.WaitGroup
	enqueued  atomic.Uint64
	processed atomic.Uint64
	rejected  atomic.Uint64
}

func NewWorkQueue(capacity, workers int, handler func(Job)) *WorkQueue {
	return &WorkQueue{
		ch:      make(chan Job, capacity),
		workers: workers,
		handler: handler,
	}
}

func (q *WorkQueue) Start() {
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go func() {
			defer q.wg.Done()
			for job := range q.ch {
				q.handler(job)
				q.processed.Add(1)
			}
		}()
	}
}

func (q *WorkQueue) Enqueue(job Job) error {
	select {
	case q.ch <- job:
		q.enqueued.Add(1)
		return nil
	default:
		q.rejected.Add(1)
		return errors.New("queue full")
	}
}

func (q *WorkQueue) Stop() {
	q.stopOnce.Do(func() { close(q.ch) })
	q.wg.Wait()
}

func (q *WorkQueue) Stats() map[string]any {
	return map[string]any{
		"capacity":  cap(q.ch),
		"depth":     len(q.ch),
		"workers":   q.workers,
		"enqueued":  q.enqueued.Load(),
		"processed": q.processed.Load(),
		"rejected":  q.rejected.Load(),
	}
}
