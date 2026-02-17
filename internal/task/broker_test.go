package task

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBroker_SubscribeAndPublish(t *testing.T) {
	broker := NewBroker(nil)
	ch := broker.Subscribe()
	defer broker.Unsubscribe(ch)

	event := Event{Type: EventTaskCreated, RepoID: "repo_123"}
	broker.Publish(context.Background(), event)

	select {
	case received := <-ch:
		assert.Equal(t, EventTaskCreated, received.Type)
		assert.Equal(t, "repo_123", received.RepoID)
	case <-time.After(time.Second):
		require.Fail(t, "timed out waiting for event")
	}
}

func TestBroker_MultipleSubscribers(t *testing.T) {
	broker := NewBroker(nil)
	ch1 := broker.Subscribe()
	ch2 := broker.Subscribe()
	defer broker.Unsubscribe(ch1)
	defer broker.Unsubscribe(ch2)

	event := Event{Type: EventTaskUpdated, RepoID: "repo_456"}
	broker.Publish(context.Background(), event)

	for i, ch := range []chan Event{ch1, ch2} {
		select {
		case received := <-ch:
			assert.Equal(t, EventTaskUpdated, received.Type, "subscriber %d: event type mismatch", i)
		case <-time.After(time.Second):
			require.Fail(t, "timed out waiting for event", "subscriber %d", i)
		}
	}
}

func TestBroker_Unsubscribe(t *testing.T) {
	broker := NewBroker(nil)
	ch := broker.Subscribe()
	broker.Unsubscribe(ch)

	// Channel should be closed
	_, ok := <-ch
	assert.False(t, ok, "expected channel to be closed after unsubscribe")
}

func TestBroker_Receive(t *testing.T) {
	broker := NewBroker(nil)
	ch := broker.Subscribe()
	defer broker.Unsubscribe(ch)

	event := Event{Type: EventLogsAppended, TaskID: NewTaskID()}
	broker.Receive(event)

	select {
	case received := <-ch:
		assert.Equal(t, EventLogsAppended, received.Type)
	case <-time.After(time.Second):
		require.Fail(t, "timed out waiting for event")
	}
}

type mockNotifier struct {
	payloads [][]byte
}

func (n *mockNotifier) Notify(_ context.Context, payload []byte) error {
	n.payloads = append(n.payloads, payload)
	return nil
}

func TestBroker_PublishWithNotifier(t *testing.T) {
	notifier := &mockNotifier{}
	broker := NewBroker(notifier)
	ch := broker.Subscribe()
	defer broker.Unsubscribe(ch)

	event := Event{Type: EventTaskCreated, RepoID: "repo_123"}
	broker.Publish(context.Background(), event)

	// When notifier is set, events go through notifier, not directly to subscribers
	assert.Len(t, notifier.payloads, 1)

	// Subscriber should NOT receive directly (goes through notifier)
	select {
	case <-ch:
		assert.Fail(t, "expected no direct event when notifier is set")
	case <-time.After(50 * time.Millisecond):
		// Expected: event goes to notifier instead
	}
}

func TestBroker_FanOutDropsOnFullBuffer(t *testing.T) {
	broker := NewBroker(nil)
	ch := broker.Subscribe()
	defer broker.Unsubscribe(ch)

	// Fill the subscriber buffer (capacity 64)
	for i := 0; i < 64; i++ {
		broker.Publish(context.Background(), Event{Type: EventTaskCreated})
	}

	// This should not block even though the buffer is full
	done := make(chan struct{})
	go func() {
		broker.Publish(context.Background(), Event{Type: EventTaskUpdated})
		close(done)
	}()

	select {
	case <-done:
		// Good: publish did not block
	case <-time.After(time.Second):
		require.Fail(t, "Publish blocked on full subscriber buffer")
	}
}

func TestEventConstants(t *testing.T) {
	assert.Equal(t, "task_created", EventTaskCreated)
	assert.Equal(t, "task_updated", EventTaskUpdated)
	assert.Equal(t, "logs_appended", EventLogsAppended)
}
