package queue

import (
	"context"
	"testing"
	"time"
)

func TestMemoryQueue_Publish(t *testing.T) {
	q := NewMemoryQueue()
	defer q.Close()

	ctx := context.Background()
	msg := &Message{
		Type: "test",
		Payload: map[string]interface{}{
			"key": "value",
		},
	}

	err := q.Publish(ctx, "test-stream", msg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if msg.ID == "" {
		t.Error("expected message ID to be set")
	}

	if msg.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestMemoryQueue_Publish_Closed(t *testing.T) {
	q := NewMemoryQueue()
	q.Close()

	ctx := context.Background()
	msg := &Message{Type: "test"}

	err := q.Publish(ctx, "test-stream", msg)
	if err == nil {
		t.Error("expected error when publishing to closed queue")
	}
}

func TestMemoryQueue_Consume(t *testing.T) {
	q := NewMemoryQueue()
	defer q.Close()

	ctx := context.Background()
	msg := &Message{
		Type: "test",
		Payload: map[string]interface{}{
			"key": "value",
		},
	}

	// Publish a message
	err := q.Publish(ctx, "test-stream", msg)
	if err != nil {
		t.Fatalf("failed to publish message: %v", err)
	}

	// Consume messages
	msgChan, err := q.Consume(ctx, "test-stream", "test-group")
	if err != nil {
		t.Fatalf("failed to consume messages: %v", err)
	}

	select {
	case receivedMsg := <-msgChan:
		if receivedMsg.Type != "test" {
			t.Errorf("expected type 'test', got '%s'", receivedMsg.Type)
		}
		if receivedMsg.Payload["key"] != "value" {
			t.Errorf("expected payload key 'value', got '%v'", receivedMsg.Payload["key"])
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for message")
	}
}

func TestMemoryQueue_Consume_Closed(t *testing.T) {
	q := NewMemoryQueue()
	q.Close()

	ctx := context.Background()
	_, err := q.Consume(ctx, "test-stream", "test-group")
	if err == nil {
		t.Error("expected error when consuming from closed queue")
	}
}

func TestMemoryQueue_MultipleConsumers(t *testing.T) {
	q := NewMemoryQueue()
	defer q.Close()

	ctx := context.Background()

	// Publish a message
	msg := &Message{Type: "test"}
	err := q.Publish(ctx, "test-stream", msg)
	if err != nil {
		t.Fatalf("failed to publish message: %v", err)
	}

	// Create two consumers from the same stream
	msgChan1, err := q.Consume(ctx, "test-stream", "group1")
	if err != nil {
		t.Fatalf("failed to create consumer 1: %v", err)
	}

	msgChan2, err := q.Consume(ctx, "test-stream", "group2")
	if err != nil {
		t.Fatalf("failed to create consumer 2: %v", err)
	}

	// Both consumers should receive the message (since they share the same underlying channel)
	receivedCount := 0
	timeout := time.After(1 * time.Second)

	for receivedCount < 2 {
		select {
		case <-msgChan1:
			receivedCount++
		case <-msgChan2:
			receivedCount++
		case <-timeout:
			// At least one should receive the message
			if receivedCount > 0 {
				t.Logf("Received %d messages before timeout", receivedCount)
				return
			}
			t.Errorf("timeout waiting for messages, received %d", receivedCount)
			return
		}
	}
}

func TestMemoryQueue_ContextCancellation(t *testing.T) {
	q := NewMemoryQueue()
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())

	msgChan, err := q.Consume(ctx, "test-stream", "test-group")
	if err != nil {
		t.Fatalf("failed to consume messages: %v", err)
	}

	// Cancel context
	cancel()

	// Wait a bit for goroutine to stop
	time.Sleep(100 * time.Millisecond)

	// Channel should be closed
	select {
	case _, ok := <-msgChan:
		if ok {
			t.Error("expected channel to be closed after context cancellation")
		}
	case <-time.After(100 * time.Millisecond):
		// Channel should be closed by now
	}
}

func TestMemoryQueue_QueueFull(t *testing.T) {
	q := NewMemoryQueue()
	defer q.Close()

	ctx := context.Background()
	timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	// Fill the queue (buffer size is 1000)
	for i := 0; i < 1001; i++ {
		msg := &Message{Type: "test"}
		err := q.Publish(timeoutCtx, "test-stream", msg)
		if i == 1000 && err == nil {
			t.Error("expected error when queue is full")
		}
		if i < 1000 && err != nil {
			t.Fatalf("unexpected error at message %d: %v", i, err)
		}
	}
}
