package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Message represents a task in the queue
type Message struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt time.Time              `json:"created_at"`
}

// Queue represents a message queue interface
type Queue interface {
	Publish(ctx context.Context, stream string, message *Message) error
	Consume(ctx context.Context, stream string, consumerGroup string) (<-chan *Message, error)
	Close() error
}

// MemoryQueue implements Queue using in-memory channels
// TODO: Replace with Redis or RabbitMQ for production distributed systems
type MemoryQueue struct {
	mu      sync.RWMutex
	streams map[string]chan *Message
	closed  bool
}

// NewMemoryQueue creates a new in-memory queue
func NewMemoryQueue() *MemoryQueue {
	return &MemoryQueue{
		streams: make(map[string]chan *Message),
	}
}

// Publish adds a message to a stream
func (q *MemoryQueue) Publish(ctx context.Context, stream string, message *Message) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return fmt.Errorf("queue is closed")
	}

	if message.ID == "" {
		message.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	message.CreatedAt = time.Now()

	ch, exists := q.streams[stream]
	if !exists {
		ch = make(chan *Message, 1000)
		q.streams[stream] = ch
	}

	select {
	case ch <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("queue full for stream: %s", stream)
	}
}

// Consume reads messages from a stream
func (q *MemoryQueue) Consume(ctx context.Context, stream string, consumerGroup string) (<-chan *Message, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return nil, fmt.Errorf("queue is closed")
	}

	ch, exists := q.streams[stream]
	if !exists {
		ch = make(chan *Message, 1000)
		q.streams[stream] = ch
	}

	outChan := make(chan *Message, 100)

	go func() {
		defer close(outChan)
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				// Deep copy message
				data, _ := json.Marshal(msg)
				var msgCopy Message
				json.Unmarshal(data, &msgCopy)

				select {
				case outChan <- &msgCopy:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return outChan, nil
}

// Close closes the queue
func (q *MemoryQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return nil
	}

	q.closed = true
	for _, ch := range q.streams {
		close(ch)
	}
	q.streams = make(map[string]chan *Message)

	return nil
}
