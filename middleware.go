package generics

import (
	"context"
	"sync"
	"time"
)

// Task represents a unit of work to be processed.
type Task struct {
	ID           string
	Data         interface{}
	AttemptCount int
}

// AlarmFunc is invoked when a task is moved to the dead letter queue due to a timeout.
type AlarmFunc func(task Task, reason string)

// Queue is a thread-safe FIFO task queue.
type Queue struct {
	mu    sync.Mutex
	tasks []Task
}

// Enqueue adds a task to the queue.
func (q *Queue) Enqueue(task Task) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks = append(q.tasks, task)
}

// Dequeue removes and returns the next task from the queue.
// The second return value is false when the queue is empty.
func (q *Queue) Dequeue() (Task, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.tasks) == 0 {
		return Task{}, false
	}
	task := q.tasks[0]
	q.tasks = q.tasks[1:]
	return task, true
}

// Len returns the number of tasks in the queue.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.tasks)
}

// DeadLetterQueue is a thread-safe store for tasks that have permanently failed processing.
type DeadLetterQueue struct {
	mu    sync.Mutex
	tasks []Task
}

// Add puts a task into the dead letter queue.
func (dlq *DeadLetterQueue) Add(task Task) {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	dlq.tasks = append(dlq.tasks, task)
}

// Tasks returns a snapshot of all tasks in the dead letter queue.
func (dlq *DeadLetterQueue) Tasks() []Task {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	snapshot := make([]Task, len(dlq.tasks))
	copy(snapshot, dlq.tasks)
	return snapshot
}

// Middleware processes tasks from a queue with timeout and retry logic.
// On the first timeout the task is re-enqueued for a retry.
// If the task times out again it is moved to the dead letter queue and the
// alarm function is called. The alarm is intentionally NOT called on a first
// timeout that results in a successful retry.
type Middleware struct {
	queue       *Queue
	dlq         *DeadLetterQueue
	timeout     time.Duration
	maxAttempts int
	alarm       AlarmFunc
}

// NewMiddleware creates a Middleware. maxAttempts controls how many times a
// task may be attempted before being sent to the DLQ; it must be at least 1.
func NewMiddleware(queue *Queue, dlq *DeadLetterQueue, timeout time.Duration, maxAttempts int, alarm AlarmFunc) *Middleware {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return &Middleware{
		queue:       queue,
		dlq:         dlq,
		timeout:     timeout,
		maxAttempts: maxAttempts,
		alarm:       alarm,
	}
}

// Process runs handler for the given task under a deadline.
// Timeout behaviour:
//   - attempt < maxAttempts → task is re-enqueued, no alarm
//   - attempt == maxAttempts → task is moved to the DLQ and the alarm fires
//
// Handlers should respect context cancellation to avoid goroutine leaks: if a
// handler ignores ctx.Done() and blocks indefinitely the spawned goroutine will
// not be reclaimed until the handler eventually returns.
func (m *Middleware) Process(task Task, handler func(ctx context.Context, task Task) error) error {
	task.AttemptCount++

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- handler(ctx, task)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if task.AttemptCount >= m.maxAttempts {
			m.dlq.Add(task)
			if m.alarm != nil {
				m.alarm(task, "task timed out and exceeded maximum attempts")
			}
		} else {
			m.queue.Enqueue(task)
		}
		return ctx.Err()
	}
}
