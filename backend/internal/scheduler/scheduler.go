package scheduler

import (
	"context"
	"log/slog"
	"time"

	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/storage"
	"corp-messenger/backend/internal/websocket"

	"github.com/google/uuid"
)

// Scheduler handles scheduled messages
type Scheduler struct {
	storage      storage.Storage
	hub          *websocket.Hub
	enabled      bool
	logger       *slog.Logger
	restartCount int
	maxRestarts  int
}

// NewScheduler creates a new scheduler
func NewScheduler(storage storage.Storage, hub *websocket.Hub, enabled bool, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		storage:      storage,
		hub:          hub,
		enabled:      enabled,
		logger:       logger,
		restartCount: 0,
		maxRestarts:  5, // Maximum 5 restarts before giving up
	}
}

// Start begins the scheduler process
func (s *Scheduler) Start(ctx context.Context) {
	if !s.enabled {
		s.logger.Info("Scheduler disabled, not starting")
		return
	}

	s.logger.Info("Starting scheduler")

	// Start periodic check for scheduled messages
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.restartCount++
				if s.restartCount <= s.maxRestarts {
					s.logger.Error("Scheduler goroutine panicked, restarting", "panic", r, "restart_count", s.restartCount, "max_restarts", s.maxRestarts)
					// Restart the scheduler after a panic with exponential backoff
					backoff := time.Duration(s.restartCount) * time.Second
					time.Sleep(backoff)
					go s.processScheduledMessages(ctx)
				} else {
					s.logger.Error("Scheduler goroutine panicked, max restarts reached, giving up", "panic", r, "restart_count", s.restartCount)
				}
			}
		}()
		s.processScheduledMessages(ctx)
	}()
}

// processScheduledMessages checks for and processes scheduled messages
func (s *Scheduler) processScheduledMessages(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Stopping scheduler")
			return
		case <-ticker.C:
			s.checkAndSendScheduledMessages(ctx)
		}
	}
}

// retryFailedScheduledMessages retries failed scheduled messages
func (s *Scheduler) retryFailedScheduledMessages(ctx context.Context) {
	failures, err := s.storage.GetSchedulerFailures(ctx, 10)
	if err != nil {
		s.logger.Error("Failed to get scheduler failures", "error", err)
		return
	}

	if len(failures) == 0 {
		return
	}

	s.logger.Info("Retrying failed scheduled messages", "count", len(failures))

	for _, failure := range failures {
		messageID, ok := failure["message_id"].(uuid.UUID)
		if !ok {
			s.logger.Error("Invalid message_id type in failure record", "failure", failure)
			continue
		}

		// Get the message
		msg, err := s.storage.GetMessageByID(ctx, messageID)
		if err != nil {
			s.logger.Error("Failed to get message for retry", "message_id", messageID, "error", err)
			continue
		}
		if msg == nil {
			s.logger.Info("Message not found, resolving failure", "message_id", messageID)
			s.storage.ResolveSchedulerFailure(ctx, messageID)
			continue
		}

		// Retry updating the message
		err = s.updateScheduledMessage(ctx, msg)
		if err != nil {
			s.logger.Error("Retry failed for message", "message_id", messageID, "error", err)
			continue
		}

		// Success - resolve the failure
		s.storage.ResolveSchedulerFailure(ctx, messageID)
		s.logger.Info("Successfully retried message", "message_id", messageID)

		// Broadcast the message to WebSocket clients
		s.hub.BroadcastToChat(&websocket.BroadcastMessage{
			Type:   "new_message",
			ChatID: msg.ChatID,
			Payload: map[string]interface{}{
				"id":         msg.ID,
				"chat_id":    msg.ChatID,
				"sender_id":  msg.SenderID,
				"content":    msg.Content,
				"type":       msg.Type,
				"created_at": msg.CreatedAt,
			},
		})
	}
}

// checkAndSendScheduledMessages checks for messages that are due to be sent
func (s *Scheduler) checkAndSendScheduledMessages(ctx context.Context) {
	// First, retry failed scheduled messages
	s.retryFailedScheduledMessages(ctx)

	messages, err := s.storage.GetScheduledMessages(ctx)
	if err != nil {
		s.logger.Error("Failed to get scheduled messages", "error", err)
		return
	}

	if len(messages) == 0 {
		return
	}

	s.logger.Info("Found scheduled messages to send", "count", len(messages))

	for _, msg := range messages {
		// Update the message to clear scheduled_at (making it a normal message)
		err := s.updateScheduledMessage(ctx, msg)
		if err != nil {
			s.logger.Error("Failed to update scheduled message, adding to failures", "message_id", msg.ID, "error", err)
			// Add to failure tracking with exponential backoff
			if dlqErr := s.storage.AddToSchedulerFailures(ctx, msg.ID, msg.ChatID, err.Error()); dlqErr != nil {
				s.logger.Error("Failed to add message to failure tracking", "message_id", msg.ID, "error", dlqErr)
			}
			continue
		}

		// Broadcast the message to WebSocket clients
		s.hub.BroadcastToChat(&websocket.BroadcastMessage{
			Type:   "new_message",
			ChatID: msg.ChatID,
			Payload: map[string]interface{}{
				"id":         msg.ID,
				"chat_id":    msg.ChatID,
				"sender_id":  msg.SenderID,
				"type":       msg.Type,
				"content":    msg.Content,
				"created_at": msg.CreatedAt,
			},
		})

		s.logger.Info("Sent scheduled message", "message_id", msg.ID)
	}
}

// updateScheduledMessage updates a scheduled message to be sent immediately
func (s *Scheduler) updateScheduledMessage(ctx context.Context, msg *models.Message) error {
	// Clear scheduled_at to mark as sent
	msg.ScheduledAt = nil
	return s.storage.UpdateMessage(ctx, msg)
}
