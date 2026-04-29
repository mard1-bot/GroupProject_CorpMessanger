package xmppsync

import (
	"context"
	"log/slog"
	"time"

	"corp-messenger/backend/internal/ejabberd"
	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/storage"

	"github.com/google/uuid"
)

// ReconciliationService handles reconciliation of XMPP state with PostgreSQL
type ReconciliationService struct {
	storage storage.Storage
	xmpp    ejabberd.Client
	enabled bool
	logger  *slog.Logger
}

// NewReconciliationService creates a new reconciliation service
func NewReconciliationService(storage storage.Storage, xmpp ejabberd.Client, enabled bool, logger *slog.Logger) *ReconciliationService {
	return &ReconciliationService{
		storage: storage,
		xmpp:    xmpp,
		enabled: enabled,
		logger:  logger,
	}
}

// Start begins the reconciliation process
func (s *ReconciliationService) Start(ctx context.Context) {
	if !s.enabled {
		s.logger.Info("XMPP reconciliation service disabled, not starting")
		return
	}

	s.logger.Info("Starting XMPP reconciliation service")

	// Start periodic reconciliation
	go s.processReconciliation(ctx)
}

// processReconciliation periodically checks for chats needing reconciliation
func (s *ReconciliationService) processReconciliation(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Stopping XMPP reconciliation service")
			return
		case <-ticker.C:
			s.checkAndReconcile(ctx)
		}
	}
}

// checkAndReconcile processes chats that need reconciliation
func (s *ReconciliationService) checkAndReconcile(ctx context.Context) {
	reconciliations, err := s.storage.GetChatsNeedingReconciliation(ctx, 10)
	if err != nil {
		s.logger.Error("Failed to get chats needing reconciliation", "error", err)
		return
	}

	if len(reconciliations) == 0 {
		return
	}

	s.logger.Info("Found chats needing reconciliation", "count", len(reconciliations))

	for _, rec := range reconciliations {
		chatID := rec["chat_id"].(uuid.UUID)
		reason := rec["reason"].(string)
		attempts := rec["attempts"].(int)
		maxAttempts := rec["max_attempts"].(int)

		// Check if we've exceeded max attempts
		if attempts >= maxAttempts {
			s.logger.Warn("Reconciliation exceeded max attempts, giving up", "chat_id", chatID, "reason", reason, "attempts", attempts)
			continue
		}

		// Attempt reconciliation based on reason
		err := s.reconcileChat(ctx, chatID, reason, rec)
		if err != nil {
			s.logger.Error("Reconciliation failed", "chat_id", chatID, "reason", reason, "error", err)
			// Increment attempt count
			// Note: This would require a method to increment attempts, for now just continue
			continue
		}

		// Resolve the reconciliation record
		err = s.storage.ResolveReconciliation(ctx, chatID, reason)
		if err != nil {
			s.logger.Error("Failed to resolve reconciliation", "chat_id", chatID, "reason", reason, "error", err)
		} else {
			s.logger.Info("Successfully reconciled chat", "chat_id", chatID, "reason", reason)
		}
	}
}

// reconcileChat attempts to reconcile a specific chat based on the failure reason
func (s *ReconciliationService) reconcileChat(ctx context.Context, chatID uuid.UUID, reason string, rec map[string]interface{}) error {
	switch reason {
	case "room_not_created":
		return s.reconcileRoomNotCreated(ctx, chatID, rec)
	case "member_not_added":
		return s.reconcileMemberNotAdded(ctx, chatID, rec)
	case "member_not_removed":
		return s.reconcileMemberNotRemoved(ctx, chatID, rec)
	case "room_not_updated":
		return s.reconcileRoomNotUpdated(ctx, chatID, rec)
	case "room_not_destroyed":
		return s.reconcileRoomNotDestroyed(ctx, chatID, rec)
	default:
		s.logger.Warn("Unknown reconciliation reason", "chat_id", chatID, "reason", reason)
		return nil
	}
}

// reconcileRoomNotCreated attempts to create an XMPP room that failed initially
func (s *ReconciliationService) reconcileRoomNotCreated(ctx context.Context, chatID uuid.UUID, rec map[string]interface{}) error {
	details := rec["details"].(map[string]interface{})
	title, _ := details["title"].(string)
	creatorID, _ := details["creator_id"].(uuid.UUID)

	if creatorID == uuid.Nil {
		return s.storage.ResolveReconciliation(ctx, chatID, "room_not_created")
	}

	err := s.xmpp.CreateChatRoom(chatID, title, creatorID)
	if err != nil {
		return err
	}

	// After creating room, add members
	members, err := s.storage.GetChatMembers(ctx, chatID)
	if err != nil {
		return err
	}

	for _, member := range members {
		var role string
		switch member.Role {
		case models.ChatRoleOwner:
			role = "owner"
		case models.ChatRoleAdmin:
			role = "admin"
		default:
			role = "member"
		}
		_ = s.xmpp.AddMemberToRoom(chatID, member.UserID, role) // Best effort
	}

	return nil
}

// reconcileMemberNotAdded attempts to add a member to an XMPP room
func (s *ReconciliationService) reconcileMemberNotAdded(ctx context.Context, chatID uuid.UUID, rec map[string]interface{}) error {
	details := rec["details"].(map[string]interface{})
	userID, _ := details["user_id"].(uuid.UUID)
	role, _ := details["role"].(string)

	if userID == uuid.Nil {
		return s.storage.ResolveReconciliation(ctx, chatID, "member_not_added")
	}

	if role == "" {
		role = "member"
	}

	return s.xmpp.AddMemberToRoom(chatID, userID, role)
}

// reconcileMemberNotRemoved attempts to remove a member from an XMPP room
func (s *ReconciliationService) reconcileMemberNotRemoved(ctx context.Context, chatID uuid.UUID, rec map[string]interface{}) error {
	details := rec["details"].(map[string]interface{})
	userID, _ := details["user_id"].(uuid.UUID)

	if userID == uuid.Nil {
		return s.storage.ResolveReconciliation(ctx, chatID, "member_not_removed")
	}

	return s.xmpp.RemoveMemberFromRoom(chatID, userID)
}

// reconcileRoomNotUpdated attempts to update an XMPP room title
func (s *ReconciliationService) reconcileRoomNotUpdated(ctx context.Context, chatID uuid.UUID, rec map[string]interface{}) error {
	details := rec["details"].(map[string]interface{})
	title, _ := details["title"].(string)

	if title == "" {
		return s.storage.ResolveReconciliation(ctx, chatID, "room_not_updated")
	}

	return s.xmpp.UpdateChatRoomTitle(chatID, title)
}

// reconcileRoomNotDestroyed attempts to destroy an XMPP room
func (s *ReconciliationService) reconcileRoomNotDestroyed(_ context.Context, chatID uuid.UUID, _ map[string]interface{}) error {
	return s.xmpp.DestroyChatRoom(chatID)
}
