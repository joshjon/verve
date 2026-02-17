package task

import (
	"context"
	"testing"
	"time"
)

func TestBroker_SubscribeAndPublish(t *testing.T) {
	broker := NewBroker(nil)
	ch := broker.Subscribe()
	defer broker.Unsubscribe(ch)

	event := Event{Type: EventTaskCreated, RepoID: "repo_123"}
	broker.Publish(context.Background(), event)

	select {
	case received := <-ch:
		if received.Type != EventTaskCreated {
			t.Errorf("expected event type %s, got %s", EventTaskCreated, received.Type)
		}
		if received.RepoID != "repo_123" {
			t.Errorf("expected repo_id repo_123, got %s", received.RepoID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
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
			if received.Type != EventTaskUpdated {
				t.Errorf("subscriber %d: expected event type %s, got %s", i, EventTaskUpdated, received.Type)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out waiting for event", i)
		}
	}
}

func TestBroker_Unsubscribe(t *testing.T) {
	broker := NewBroker(nil)
	ch := broker.Subscribe()
	broker.Unsubscribe(ch)

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after unsubscribe")
	}
}

func TestBroker_Receive(t *testing.T) {
	broker := NewBroker(nil)
	ch := broker.Subscribe()
	defer broker.Unsubscribe(ch)

	event := Event{Type: EventLogsAppended, TaskID: NewTaskID()}
	broker.Receive(event)

	select {
	case received := <-ch:
		if received.Type != EventLogsAppended {
			t.Errorf("expected event type %s, got %s", EventLogsAppended, received.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
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
	if len(notifier.payloads) != 1 {
		t.Errorf("expected 1 notifier payload, got %d", len(notifier.payloads))
	}

	// Subscriber should NOT receive directly (goes through notifier)
	select {
	case <-ch:
		t.Error("expected no direct event when notifier is set")
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
		t.Fatal("Publish blocked on full subscriber buffer")
	}
}

func TestEventConstants(t *testing.T) {
	if EventTaskCreated != "task_created" {
		t.Errorf("expected 'task_created', got %s", EventTaskCreated)
	}
	if EventTaskUpdated != "task_updated" {
		t.Errorf("expected 'task_updated', got %s", EventTaskUpdated)
	}
	if EventLogsAppended != "logs_appended" {
		t.Errorf("expected 'logs_appended', got %s", EventLogsAppended)
	}
}
