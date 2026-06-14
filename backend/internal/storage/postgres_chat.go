package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"time"

	"github.com/google/uuid"

	"corp-messenger/backend/internal/models"
)

func (s *PostgresStorage) UsersShareChat(ctx context.Context, userID1, userID2 uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM chat_members cm1
			JOIN chat_members cm2 ON cm1.chat_id = cm2.chat_id
			WHERE cm1.user_id = $1 AND cm2.user_id = $2
		)
	`
	var exists bool
	err := s.db.QueryRowContext(ctx, query, userID1, userID2).Scan(&exists)
	return exists, err
}

func (s *PostgresStorage) CreateChat(ctx context.Context, chat *models.Chat) error {
	query := `
		INSERT INTO chats (type, title, description, avatar, creator_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	return s.db.QueryRowContext(ctx, query,
		chat.Type, chat.Title, chat.Description, chat.Avatar, chat.CreatorID,
	).Scan(&chat.ID, &chat.CreatedAt, &chat.UpdatedAt)
}

func (s *PostgresStorage) findExistingDirectChat(ctx context.Context, userID1, userID2 uuid.UUID) (uuid.UUID, error) {
	query := `
		SELECT c.id FROM chats c
		JOIN chat_members cm1 ON c.id = cm1.chat_id AND cm1.user_id = $1
		JOIN chat_members cm2 ON c.id = cm2.chat_id AND cm2.user_id = $2
		WHERE c.type = 'direct'
		LIMIT 1
	`
	var chatID uuid.UUID
	err := s.db.QueryRowContext(ctx, query, userID1, userID2).Scan(&chatID)
	if err == sql.ErrNoRows {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, err
	}
	return chatID, nil
}

func (s *PostgresStorage) CreateChatWithMembers(ctx context.Context, chat *models.Chat, members []*models.ChatMember) error {
	// For direct chats, check if one already exists between these users
	if chat.Type == models.ChatTypeDirect && len(members) == 2 {
		existingChatID, err := s.findExistingDirectChat(ctx, members[0].UserID, members[1].UserID)
		if err != nil {
			return fmt.Errorf("check existing direct chat: %w", err)
		}
		if existingChatID != uuid.Nil {
			return fmt.Errorf("direct chat already exists")
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// Handle rollback with error check
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
			}
		}
	}()

	// Create chat
	chatQuery := `
		INSERT INTO chats (type, title, description, avatar, creator_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	err = tx.QueryRowContext(ctx, chatQuery,
		chat.Type, chat.Title, chat.Description, chat.Avatar, chat.CreatorID,
	).Scan(&chat.ID, &chat.CreatedAt, &chat.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create chat: %w", err)
	}

	// Add members
	memberQuery := `
		INSERT INTO chat_members (chat_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, user_id) DO UPDATE SET role = $3
	`
	for _, member := range members {
		member.ChatID = chat.ID
		_, err = tx.ExecContext(ctx, memberQuery, member.ChatID, member.UserID, member.Role)
		if err != nil {
			return fmt.Errorf("add member %s: %w", member.UserID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *PostgresStorage) GetChatByID(ctx context.Context, id uuid.UUID) (*models.Chat, error) {
	chat := &models.Chat{}
	query := `
		SELECT id, type, title, description, avatar, creator_id, created_at, updated_at
		FROM chats WHERE id = $1
	`
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&chat.ID, &chat.Type, &chat.Title, &chat.Description, &chat.Avatar,
		&chat.CreatorID, &chat.CreatedAt, &chat.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return chat, nil
}

func (s *PostgresStorage) UpdateChat(ctx context.Context, chat *models.Chat) error {
	query := `
		UPDATE chats
		SET title = $1, description = $2, avatar = $3, updated_at = NOW()
		WHERE id = $4
		RETURNING updated_at
	`
	return s.db.QueryRowContext(ctx, query, chat.Title, chat.Description, chat.Avatar, chat.ID).Scan(&chat.UpdatedAt)
}

func (s *PostgresStorage) GetUserChats(ctx context.Context, userID uuid.UUID) ([]*models.Chat, error) {
	// Get chats excluding archived and soft-deleted, sorted by last message time
	query := `
		SELECT c.id, c.type, c.title, c.description, c.avatar, c.creator_id, c.created_at, c.updated_at,
		       lm.last_msg_at, lm_last_msg.content as last_message
		FROM chats c
		JOIN chat_members cm ON c.id = cm.chat_id
		LEFT JOIN (
			SELECT chat_id, MAX(created_at) AS last_msg_at
			FROM messages
			GROUP BY chat_id
		) lm ON c.id = lm.chat_id
		LEFT JOIN messages lm_last_msg ON lm_last_msg.chat_id = c.id AND lm_last_msg.created_at = lm.last_msg_at
		WHERE cm.user_id = $1 AND cm.archived = false AND cm.deleted_at IS NULL
		ORDER BY COALESCE(lm.last_msg_at, c.created_at) DESC
	`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []*models.Chat
	for rows.Next() {
		chat := &models.Chat{}
		var lastMsgAt sql.NullTime
		var lastMessage sql.NullString
		err := rows.Scan(
			&chat.ID, &chat.Type, &chat.Title, &chat.Description, &chat.Avatar,
			&chat.CreatorID, &chat.CreatedAt, &chat.UpdatedAt,
			&lastMsgAt, &lastMessage,
		)
		if err != nil {
			return nil, err
		}
		if lastMsgAt.Valid {
			chat.LastMessageAt = &lastMsgAt.Time
		}
		if lastMessage.Valid {
			chat.LastMessage = &lastMessage.String
		}
		chats = append(chats, chat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// For direct chats, get other participant info
	for _, chat := range chats {
		if chat.Type == "direct" {
			members, err := s.GetChatMembersWithUsers(ctx, chat.ID)
			if err == nil {
				chat.Members = members
			}
		}
	}

	return chats, nil
}

func (s *PostgresStorage) AddChatMember(ctx context.Context, member *models.ChatMember) error {
	// Add member
	query := `
		INSERT INTO chat_members (chat_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, user_id) DO UPDATE SET role = $3
	`
	_, err := s.db.ExecContext(ctx, query, member.ChatID, member.UserID, member.Role)
	if err != nil {
		return err
	}

	// Update chat's updated_at timestamp
	_, err = s.db.ExecContext(ctx, `UPDATE chats SET updated_at = NOW() WHERE id = $1`, member.ChatID)
	return err
}

func (s *PostgresStorage) GetChatMembers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error) {
	query := `
		SELECT chat_id, user_id, role, joined_at, last_read_at, muted, pinned, archived, deleted_at
		FROM chat_members WHERE chat_id = $1
	`
	rows, err := s.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*models.ChatMember
	for rows.Next() {
		m := &models.ChatMember{}
		err := rows.Scan(&m.ChatID, &m.UserID, &m.Role, &m.JoinedAt, &m.LastReadAt, &m.Muted, &m.Pinned, &m.Archived, &m.DeletedAt)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *PostgresStorage) GetChatMembersWithUsers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error) {
	query := `
		SELECT cm.chat_id, cm.user_id, cm.role, cm.joined_at, cm.last_read_at, cm.muted, cm.pinned, cm.archived, cm.deleted_at,
			u.id, u.email, u.username, u.first_name, u.last_name, u.middle_name, u.avatar, u.status, u.role, u.created_at, u.updated_at, u.last_online
		FROM chat_members cm
		JOIN users u ON cm.user_id = u.id
		WHERE cm.chat_id = $1
	`
	rows, err := s.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*models.ChatMember
	for rows.Next() {
		m := &models.ChatMember{}
		u := &models.User{}
		var username, middleName, avatar sql.NullString
		err := rows.Scan(
			&m.ChatID, &m.UserID, &m.Role, &m.JoinedAt, &m.LastReadAt, &m.Muted, &m.Pinned, &m.Archived, &m.DeletedAt,
			&u.ID, &u.Email, &username, &u.FirstName, &u.LastName, &middleName, &avatar, &u.Status, &u.Role, &u.CreatedAt, &u.UpdatedAt, &u.LastOnline,
		)
		if err != nil {
			return nil, err
		}
		if username.Valid {
			u.Username = &username.String
		}
		if middleName.Valid {
			u.MiddleName = &middleName.String
		}
		if avatar.Valid {
			u.Avatar = &avatar.String
		}
		m.User = u
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *PostgresStorage) MuteChat(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_members SET muted = true WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (s *PostgresStorage) UnmuteChat(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_members SET muted = false WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (s *PostgresStorage) PinChat(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_members SET pinned = true WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (s *PostgresStorage) UnpinChat(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_members SET pinned = false WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (s *PostgresStorage) ArchiveChat(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_members SET archived = true WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (s *PostgresStorage) UnarchiveChat(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_members SET archived = false WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (s *PostgresStorage) SoftDeleteChat(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_members SET deleted_at = NOW() WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (s *PostgresStorage) ClearChatHistory(ctx context.Context, chatID, userID uuid.UUID) error {
	// Mark all messages in this chat as deleted for the user
	query := `
		INSERT INTO deleted_messages (user_id, message_id)
		SELECT $1, m.id FROM messages m WHERE m.chat_id = $2
		ON CONFLICT DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, query, userID, chatID)
	return err
}

func (s *PostgresStorage) MarkChatForReconciliation(ctx context.Context, chatID uuid.UUID, reason string, details map[string]interface{}) error {
	query := `
		INSERT INTO xmpp_reconciliation (chat_id, reason, details)
		VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, reason, resolved_at) 
		DO UPDATE SET 
			details = EXCLUDED.details,
			attempts = EXCLUDED.attempts + 1,
			last_attempt_at = NOW(),
			created_at = EXCLUDED.created_at
		WHERE xmpp_reconciliation.resolved_at IS NULL
	`
	_, err := s.db.ExecContext(ctx, query, chatID, reason, details)
	return err
}

func (s *PostgresStorage) GetChatsNeedingReconciliation(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT id, chat_id, reason, details, created_at, attempts, last_attempt_at
		FROM xmpp_reconciliation
		WHERE resolved_at IS NULL AND attempts < max_attempts
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, chatID uuid.UUID
		var reason string
		var details map[string]interface{}
		var createdAt, lastAttemptAt time.Time
		var attempts int

		if err := rows.Scan(&id, &chatID, &reason, &details, &createdAt, &attempts, &lastAttemptAt); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"id":           id,
			"chat_id":      chatID,
			"reason":       reason,
			"details":      details,
			"created_at":   createdAt,
			"attempts":     attempts,
			"last_attempt": lastAttemptAt,
		})
	}
	return results, rows.Err()
}

func (s *PostgresStorage) RemoveChatMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	// Remove member and check if any members left
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
				err = fmt.Errorf("original: %w; rollback: %v", err, rbErr)
			}
		}
	}()

	// Delete the member
	deleteQuery := `DELETE FROM chat_members WHERE chat_id = $1 AND user_id = $2`
	_, err = tx.ExecContext(ctx, deleteQuery, chatID, userID)
	if err != nil {
		return false, fmt.Errorf("delete member: %w", err)
	}

	// Check remaining member count
	var count int
	countQuery := `SELECT COUNT(*) FROM chat_members WHERE chat_id = $1`
	err = tx.QueryRowContext(ctx, countQuery, chatID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("count members: %w", err)
	}

	// If no members left, delete the chat
	if count == 0 {
		_, err = tx.ExecContext(ctx, `DELETE FROM chats WHERE id = $1`, chatID)
		if err != nil {
			return false, fmt.Errorf("delete chat: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}

	return count == 0, nil
}

func (s *PostgresStorage) DeleteChat(ctx context.Context, chatID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chats WHERE id = $1`, chatID)
	return err
}

func (s *PostgresStorage) GetMessagesByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	return s.GetMessagesByChatForUser(ctx, chatID, uuid.Nil, limit, offset)
}

func (s *PostgresStorage) GetAllMessagesByChat(ctx context.Context, chatID uuid.UUID) ([]*models.Message, error) {
	query := `
		SELECT ` + messageColumns + `
		FROM messages m
		LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id
		LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id
		WHERE m.chat_id = $1
		ORDER BY m.created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		var m models.Message
		if err := scanMessageWithEncryption(rows, &m); err != nil {
			return nil, err
		}
		messages = append(messages, &m)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

func (s *PostgresStorage) GetMessagesByChatForUser(ctx context.Context, chatID, userID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	var query string
	var args []interface{}

	if userID == uuid.Nil {
		// No user filter - get all messages (for backward compatibility)
		query = `
			SELECT ` + messageColumns + `
			FROM messages m
			LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id
			LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id
			WHERE m.chat_id = $1
			ORDER BY m.created_at ASC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{chatID, limit, offset}
	} else {
		// Filter out messages deleted by this specific user
		query = `
			SELECT ` + messageColumns + `
			FROM messages m
			LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id
			LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id
			WHERE m.chat_id = $1 AND m.id NOT IN (
				SELECT message_id FROM deleted_messages WHERE user_id = $2
			)
			ORDER BY m.created_at ASC
			LIMIT $3 OFFSET $4
		`
		args = []interface{}{chatID, userID, limit, offset}
	}

	log.Printf("[DB] GetMessagesByChatForUser: chatID=%s, userID=%s, limit=%d, offset=%d", chatID, userID, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		m := &models.Message{}
		if err := scanMessageWithEncryption(rows, m); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	// Load pinned message IDs for this chat
	pinnedQuery := `SELECT message_id FROM pinned_messages WHERE chat_id = $1`
	log.Printf("[DB] Loading pinned messages for chat %s", chatID)
	pinnedRows, err := s.db.QueryContext(ctx, pinnedQuery, chatID)
	if err != nil {
		log.Printf("[DB] Failed to load pinned messages: %v", err)
		// Continue without pinned status
		return messages, rows.Err()
	}
	defer pinnedRows.Close()

	pinnedIDs := make(map[uuid.UUID]bool)
	pinnedCount := 0
	for pinnedRows.Next() {
		var messageID uuid.UUID
		if err := pinnedRows.Scan(&messageID); err != nil {
			log.Printf("[DB] Failed to scan pinned message ID: %v", err)
			continue
		}
		pinnedIDs[messageID] = true
		pinnedCount++
	}
	log.Printf("[DB] Loaded %d pinned messages for chat %s", pinnedCount, chatID)

	// Mark messages as pinned
	markedCount := 0
	for _, m := range messages {
		if pinnedIDs[m.ID] {
			m.Pinned = true
			markedCount++
		}
	}
	log.Printf("[DB] Marked %d messages as pinned out of %d total", markedCount, len(messages))

	return messages, rows.Err()
}

func (s *PostgresStorage) GetChatMember(ctx context.Context, chatID, userID uuid.UUID) (*models.ChatMember, error) {
	query := `
		SELECT chat_id, user_id, role, joined_at, last_read_at, muted, pinned, archived, deleted_at
		FROM chat_members
		WHERE chat_id = $1 AND user_id = $2
	`
	m := &models.ChatMember{}
	var lastRead *time.Time
	err := s.db.QueryRowContext(ctx, query, chatID, userID).Scan(
		&m.ChatID, &m.UserID, &m.Role, &m.JoinedAt, &lastRead, &m.Muted, &m.Pinned, &m.Archived, &m.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	m.LastReadAt = lastRead
	return m, nil
}
