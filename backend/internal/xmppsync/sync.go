package xmppsync

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"corp-messenger/backend/internal/ejabberd"
	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/storage"

	"github.com/google/uuid"
)

// SyncService handles bidirectional synchronization between XMPP and PostgreSQL
type SyncService struct {
	storage storage.Storage
	xmpp    ejabberd.Client
	enabled bool
	logger  *slog.Logger
}

// NewSyncService creates a new sync service
func NewSyncService(storage storage.Storage, xmpp ejabberd.Client, enabled bool, logger *slog.Logger) *SyncService {
	return &SyncService{
		storage: storage,
		xmpp:    xmpp,
		enabled: enabled,
		logger:  logger,
	}
}

// Start begins the synchronization process
func (s *SyncService) Start(ctx context.Context) {
	if !s.enabled {
		s.logger.Info("XMPPSync service disabled, not starting")
		return
	}

	s.logger.Info("Starting XMPPSync service")

	// Start periodic sync from PostgreSQL to XMPP
	go s.syncPostgreSQLToXMPP(ctx)

	// Start periodic sync from XMPP to PostgreSQL (polling approach)
	go s.syncXMPPToPostgreSQL(ctx)
}

// syncPostgreSQLToXMPP syncs messages from PostgreSQL to XMPP
func (s *SyncService) syncPostgreSQLToXMPP(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Stopping PostgreSQL to XMPP sync")
			return
		case <-ticker.C:
			s.checkAndSyncMessagesToXMPP(ctx)
		}
	}
}

// syncXMPPToPostgreSQL syncs messages from XMPP to PostgreSQL
func (s *SyncService) syncXMPPToPostgreSQL(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Stopping XMPP to PostgreSQL sync")
			return
		case <-ticker.C:
			s.checkAndSyncMessagesFromXMPP(ctx)
		}
	}
}

// checkAndSyncMessagesToXMPP checks for messages that need to be synced to XMPP
func (s *SyncService) checkAndSyncMessagesToXMPP(ctx context.Context) {
	batchSize := 100
	totalSynced := 0

	for {
		// Get messages that haven't been synced to XMPP
		// Note: Current storage method doesn't support offset, so we sync in batches
		// and rely on the fact that synced messages are marked and won't be returned again
		messages, err := s.storage.GetUnsyncedMessages(ctx, batchSize)
		if err != nil {
			s.logger.Error("Failed to get unsynced messages", "error", err)
			return
		}

		if len(messages) == 0 {
			break // No more messages to sync
		}

		s.logger.Info("Found messages to sync to XMPP", "count", len(messages))

		// Safety: prevent infinite loop (max 100 iterations = 10k messages)
		// Check limit BEFORE processing batch
		if totalSynced+len(messages) > 10000 {
			s.logger.Warn("Safety limit would be exceeded, stopping sync", "current_synced", totalSynced, "batch_size", len(messages), "limit", 10000)
			break
		}

		for _, msg := range messages {
			// Skip scheduled messages
			if msg.ScheduledAt != nil && msg.ScheduledAt.After(time.Now()) {
				continue
			}

			// Build sender JID using actual XMPP host
			xmppHost := s.xmpp.GetHost()
			if xmppHost == "" {
				xmppHost = "localhost"
			}
			senderJID := fmt.Sprintf("%s@%s", msg.SenderID.String(), xmppHost)

			// Send to XMPP room with retry mechanism (max 3 attempts with exponential backoff)
			xmppMsgID := uuid.New().String()
			var err error
			maxRetries := 3
			for attempt := 1; attempt <= maxRetries; attempt++ {
				err = s.xmpp.SendRoomMessage(msg.ChatID, senderJID, msg.Content)
				if err == nil {
					break
				}
				if attempt < maxRetries {
					backoff := time.Duration(attempt) * time.Second
					s.logger.Warn("Retrying XMPP message send", "message_id", msg.ID, "attempt", attempt, "max_retries", maxRetries, "backoff", backoff)
					time.Sleep(backoff)
				}
			}

			if err != nil {
				s.logger.Error("Failed to send message to XMPP after retries", "message_id", msg.ID, "max_retries", maxRetries, "error", err)
				continue
			}

			// Mark as synced
			err = s.storage.MarkMessageAsSyncedToXMPP(ctx, msg.ID, xmppMsgID)
			if err != nil {
				s.logger.Error("Failed to mark message as synced, adding to dead letter queue", "message_id", msg.ID, "error", err)
				// Add to dead letter queue for retry
				if dlqErr := s.storage.AddToXMPPSyncDeadLetter(ctx, msg.ID, msg.ChatID, err.Error()); dlqErr != nil {
					s.logger.Error("Failed to add message to dead letter queue", "message_id", msg.ID, "error", dlqErr)
				}
			} else {
				s.logger.Info("Synced message to XMPP", "message_id", msg.ID, "xmpp_message_id", xmppMsgID)
				totalSynced++
			}
		}

		// Move to next batch
		// Note: Since we can't use offset with current storage method,
		// we rely on the fact that synced messages are marked and won't be returned.
		// This means we might sync fewer than all messages if there are >100 unsynced,
		// but subsequent runs will catch the rest.
	}

	if totalSynced > 0 {
		s.logger.Info("Total synced messages to XMPP", "count", totalSynced)
	}
}

// checkAndSyncMessagesFromXMPP checks for messages from XMPP that need to be synced
func (s *SyncService) checkAndSyncMessagesFromXMPP(ctx context.Context) {
	// XMPP to PostgreSQL sync requires mod_mam (Message Archive Management) in ejabberd
	// This feature is disabled by default as it requires:
	// 1. mod_mam enabled in ejabberd configuration
	// 2. Implementation of GetRoomMessages in XMPP client
	// 3. Proper handling of XMPP message format and timestamps
	// 4. Deduplication logic to avoid syncing same messages multiple times

	s.logger.Debug("XMPP to PostgreSQL sync check skipped - requires mod_mam configuration")

	// Future implementation would:
	// 1. Get all active chats from PostgreSQL
	// 2. For each chat, query XMPP for archived messages since last sync
	// 3. Parse XMPP messages and convert to PostgreSQL format
	// 4. Check for duplicates using xmpp_message_id
	// 5. Insert new messages into PostgreSQL
	// 6. Update last_sync_timestamp for each chat
}

// SyncMessageToXMPP sends a PostgreSQL message to XMPP
func (s *SyncService) SyncMessageToXMPP(ctx context.Context, msg *models.Message, senderJID string) error {
	if !s.enabled {
		return nil
	}

	// Convert PostgreSQL message to XMPP format
	// and send via XMPP client

	// Build message content based on type
	content := msg.Content
	switch msg.Type {
	case models.MessageTypeText:
		content = msg.Content
	case models.MessageTypeImage:
		if msg.FileURL != nil {
			content = fmt.Sprintf("[Image] %s", *msg.FileURL)
		} else {
			content = "[Image]"
		}
	case models.MessageTypeFile:
		if msg.FileURL != nil {
			content = fmt.Sprintf("[File] %s", *msg.FileURL)
		} else {
			content = "[File]"
		}
	case models.MessageTypeVoice:
		if msg.FileURL != nil {
			content = fmt.Sprintf("[Voice Message] %s", *msg.FileURL)
		} else {
			content = "[Voice Message]"
		}
	case models.MessageTypeVideo:
		if msg.FileURL != nil {
			content = fmt.Sprintf("[Video] %s", *msg.FileURL)
		} else {
			content = "[Video]"
		}
	default:
		content = msg.Content
	}

	// Send to XMPP room
	err := s.xmpp.SendRoomMessage(msg.ChatID, senderJID, content)
	if err != nil {
		s.logger.Error("Failed to send message to XMPP room", "chat_id", msg.ChatID, "message_id", msg.ID, "error", err)
		return err
	}
	s.logger.Info("Synced message to XMPP room", "message_id", msg.ID, "type", msg.Type, "chat_id", msg.ChatID)

	return nil
}

// GetXMPPMessages retrieves messages from XMPP for a room
func (s *SyncService) GetXMPPMessages(roomID uuid.UUID, limit int) ([]map[string]interface{}, error) {
	if !s.enabled {
		return nil, nil
	}

	return s.xmpp.GetRoomMessages(roomID, limit)
}
