package generics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noopHandler returns nil immediately (successful processing).
func noopHandler(_ context.Context, _ Task) error {
	return nil
}

// blockingHandler blocks until its context is cancelled (simulates a timeout).
func blockingHandler(ctx context.Context, _ Task) error {
	<-ctx.Done()
	return ctx.Err()
}

// errorHandler always returns a non-timeout error.
func errorHandler(_ context.Context, _ Task) error {
	return errors.New("processing error")
}

// TestProcessSuccess verifies that a successful task leaves the queue and DLQ empty
// and does not fire the alarm.
func TestProcessSuccess(t *testing.T) {
	q := &Queue{}
	dlq := &DeadLetterQueue{}
	alarmFired := false
	alarm := func(_ Task, _ string) { alarmFired = true }

	mw := NewMiddleware(q, dlq, 100*time.Millisecond, 2, alarm)
	task := Task{ID: "t1"}

	err := mw.Process(task, noopHandler)

	assert.NoError(t, err)
	assert.Equal(t, 0, q.Len(), "queue should be empty after success")
	assert.Empty(t, dlq.Tasks(), "DLQ should be empty after success")
	assert.False(t, alarmFired, "alarm must not fire on success")
}

// TestFirstTimeoutRequeuesWithoutAlarm verifies that the first timeout re-enqueues
// the task and does NOT fire the alarm.
func TestFirstTimeoutRequeuesWithoutAlarm(t *testing.T) {
	q := &Queue{}
	dlq := &DeadLetterQueue{}
	alarmFired := false
	alarm := func(_ Task, _ string) { alarmFired = true }

	mw := NewMiddleware(q, dlq, 10*time.Millisecond, 2, alarm)
	task := Task{ID: "t1"}

	err := mw.Process(task, blockingHandler)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, 1, q.Len(), "task should be re-enqueued after first timeout")
	assert.Empty(t, dlq.Tasks(), "DLQ should be empty after first timeout")
	assert.False(t, alarmFired, "alarm must NOT fire on first timeout")
}

// TestSecondTimeoutMovesToDLQAndFiresAlarm verifies that a task that has already
// been retried once is moved to the DLQ on its second timeout and the alarm fires.
func TestSecondTimeoutMovesToDLQAndFiresAlarm(t *testing.T) {
	q := &Queue{}
	dlq := &DeadLetterQueue{}
	alarmCount := 0
	var alarmTask Task
	alarm := func(t Task, _ string) {
		alarmCount++
		alarmTask = t
	}

	mw := NewMiddleware(q, dlq, 10*time.Millisecond, 2, alarm)
	// Simulate a task that has already been attempted once.
	task := Task{ID: "t1", AttemptCount: 1}

	err := mw.Process(task, blockingHandler)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, 0, q.Len(), "task must NOT be re-enqueued when moved to DLQ")
	assert.Len(t, dlq.Tasks(), 1, "task should be in the DLQ")
	assert.Equal(t, 1, alarmCount, "alarm must fire exactly once")
	assert.Equal(t, "t1", alarmTask.ID, "alarm must reference the timed-out task")
}

// TestAlarmOnlyFiresOnDLQNotOnRetry exercises the full retry cycle and ensures
// the alarm fires exactly once – when the task is sent to the DLQ.
func TestAlarmOnlyFiresOnDLQNotOnRetry(t *testing.T) {
	q := &Queue{}
	dlq := &DeadLetterQueue{}
	alarmCount := 0
	alarm := func(_ Task, _ string) { alarmCount++ }

	mw := NewMiddleware(q, dlq, 10*time.Millisecond, 2, alarm)
	task := Task{ID: "t1"}

	// First attempt → timeout → re-queued, no alarm.
	err := mw.Process(task, blockingHandler)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, 0, alarmCount, "alarm must not fire after first timeout")

	// Retrieve the re-queued task and attempt it again.
	retried, ok := q.Dequeue()
	require.True(t, ok, "task should have been re-enqueued")

	// Second attempt → timeout → DLQ, alarm fires.
	err = mw.Process(retried, blockingHandler)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, 1, alarmCount, "alarm must fire exactly once on DLQ move")
	assert.Len(t, dlq.Tasks(), 1)
}

// TestNonTimeoutErrorDoesNotAlarm verifies that a non-timeout handler error
// is returned as-is without touching the DLQ or firing the alarm.
func TestNonTimeoutErrorDoesNotAlarm(t *testing.T) {
	q := &Queue{}
	dlq := &DeadLetterQueue{}
	alarmFired := false
	alarm := func(_ Task, _ string) { alarmFired = true }

	mw := NewMiddleware(q, dlq, 100*time.Millisecond, 2, alarm)
	task := Task{ID: "t1"}

	err := mw.Process(task, errorHandler)

	assert.Error(t, err)
	assert.NotErrorIs(t, err, context.DeadlineExceeded)
	assert.Empty(t, dlq.Tasks(), "DLQ should be empty for non-timeout errors")
	assert.False(t, alarmFired, "alarm must not fire for non-timeout errors")
}

// TestQueueEnqueueDequeue verifies basic queue operations.
func TestQueueEnqueueDequeue(t *testing.T) {
	q := &Queue{}
	assert.Equal(t, 0, q.Len())

	q.Enqueue(Task{ID: "a"})
	q.Enqueue(Task{ID: "b"})
	assert.Equal(t, 2, q.Len())

	got, ok := q.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, "a", got.ID)

	got, ok = q.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, "b", got.ID)

	_, ok = q.Dequeue()
	assert.False(t, ok, "dequeue on empty queue should return false")
}

// TestDeadLetterQueueAdd verifies that tasks accumulate in the DLQ.
func TestDeadLetterQueueAdd(t *testing.T) {
	dlq := &DeadLetterQueue{}
	assert.Empty(t, dlq.Tasks())

	dlq.Add(Task{ID: "x"})
	dlq.Add(Task{ID: "y"})
	assert.Len(t, dlq.Tasks(), 2)
	assert.Equal(t, "x", dlq.Tasks()[0].ID)
	assert.Equal(t, "y", dlq.Tasks()[1].ID)
}
