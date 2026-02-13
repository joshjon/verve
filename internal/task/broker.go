package task

import "sync"

// EventType identifies the kind of SSE event.
const (
	EventTaskCreated  = "task_created"
	EventTaskUpdated  = "task_updated"
	EventLogsAppended = "logs_appended"
)

// Event represents a task mutation broadcast to SSE subscribers.
type Event struct {
	Type   string `json:"type"`
	Task   *Task  `json:"task,omitempty"`
	TaskID TaskID `json:"task_id,omitempty"`
	Logs   []string `json:"logs,omitempty"`
}

// Broker fans out task events to SSE subscribers.
type Broker struct {
	mu   sync.RWMutex
	subs map[chan Event]struct{}
}

// NewBroker creates a new Broker.
func NewBroker() *Broker {
	return &Broker{subs: make(map[chan Event]struct{})}
}

// Subscribe returns a buffered channel that receives task events.
func (b *Broker) Subscribe() chan Event {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes a subscriber channel.
func (b *Broker) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
	close(ch)
}

// Publish sends an event to all subscribers. Non-blocking: drops if a
// subscriber's buffer is full.
func (b *Broker) Publish(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- event:
		default:
		}
	}
}
