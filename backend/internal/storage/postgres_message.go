package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"time"

	"github.com/google/uuid"

	"corp-messenger/backend/internal/models"

	"github.com/lib/pq"
)

func (s *PostgresStorage) CreateMessage(ctx context.Context, msg *models.Message) error {
	query := `
		INSERT INTO messages (chat_id, sender_id, type, content, file_url, reply_to, encrypted_content, encrypted_keys, location, duration, forwarded_from, forwarded_sender_name, scheduled_at, thread_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, created_at, updated_at
	`
	// Convert encrypted keys map to JSONB
	var encryptedKeysJSON interface{}
	if len(msg.EncryptedKeys) > 0 {
		data, err := json.Marshal(msg.EncryptedKeys)
		if err != nil {
			return fmt.Errorf("marshal encrypted keys: %w", err)
		}
		encryptedKeysJSON = string(data)
	}

	// Convert location to JSONB
	var locationJSON interface{}
	if msg.Location != nil {
		data, err := json.Marshal(msg.Location)
		if err != nil {
			return fmt.Errorf("marshal location: %w", err)
		}
		locationJSON = string(data)
	}

	// Convert forwarded fields to appropriate types
	var forwardedFrom *uuid.UUID
	var forwardedSenderName *string
	if msg.ForwardedFrom != nil {
		forwardedFrom = msg.ForwardedFrom
	}
	if msg.ForwardedSenderName != nil {
		forwardedSenderName = msg.ForwardedSenderName
	}

	log.Printf("[DB] CreateMessage: forwarded_from=%v, forwarded_sender_name=%v",
		forwardedFrom, forwardedSenderName)

	return s.db.QueryRowContext(ctx, query,
		msg.ChatID, msg.SenderID, msg.Type, msg.Content, msg.FileURL, msg.ReplyTo,
		msg.EncryptedContent, encryptedKeysJSON,
		locationJSON, msg.Duration,
		forwardedFrom, forwardedSenderName,
		msg.ScheduledAt, msg.ThreadID).Scan(&msg.ID, &msg.CreatedAt, &msg.UpdatedAt)
}

func (s *PostgresStorage) GetMessageByID(ctx context.Context, id uuid.UUID) (*models.Message, error) {
	msg := &models.Message{}
	query := `
		SELECT ` + messageColumns + `
		FROM messages m
		LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id
		LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id
		WHERE m.id = $1
	`
	err := scanMessageWithEncryption(
		s.db.QueryRowContext(ctx, query, id),
		msg,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return msg, nil
}

// scanMessageWithEncryption scans a message row including encryption fields
func scanMessageWithEncryption(rows interface{ Scan(...interface{}) error }, m *models.Message) error {
	var replyTo sql.NullString
	var fileURL sql.NullString
	var encryptedContent sql.NullString
	var encryptedKeysJSON []byte
	var locationJSON []byte
	var duration sql.NullFloat64
	var forwardedFrom sql.NullString
	var forwardedSenderName sql.NullString
	var scheduledAt sql.NullTime
	var threadID sql.NullString
	var syncedToXMPP sql.NullBool
	var xmppMessageID sql.NullString
	var replyToContent sql.NullString
	var replyToSenderName sql.NullString
	err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Type, &m.Content, &fileURL,
		&m.CreatedAt, &m.UpdatedAt, &replyTo,
		&encryptedContent, &encryptedKeysJSON,
		&locationJSON, &duration,
		&forwardedFrom, &forwardedSenderName,
		&scheduledAt, &threadID, &syncedToXMPP, &xmppMessageID,
		&replyToContent, &replyToSenderName)
	if err != nil {
		return err
	}
	if fileURL.Valid {
		m.FileURL = &fileURL.String
	}
	if replyTo.Valid {
		id, err := uuid.Parse(replyTo.String)
		if err == nil {
			m.ReplyTo = &id
		}
	}
	if replyToContent.Valid {
		m.ReplyToContent = &replyToContent.String
	}
	if replyToSenderName.Valid && replyToSenderName.String != "" {
		m.ReplyToSenderName = &replyToSenderName.String
	}
	if encryptedContent.Valid {
		m.EncryptedContent = encryptedContent.String
	}
	if encryptedKeysJSON != nil {
		if err := json.Unmarshal(encryptedKeysJSON, &m.EncryptedKeys); err != nil {
			log.Printf("[ERROR] Failed to unmarshal encrypted keys: %v", err)
		}
	}
	if locationJSON != nil {
		if err := json.Unmarshal(locationJSON, &m.Location); err != nil {
			log.Printf("[ERROR] Failed to unmarshal location: %v", err)
		}
	}
	if duration.Valid {
		m.Duration = &duration.Float64
	}
	if forwardedFrom.Valid {
		id, err := uuid.Parse(forwardedFrom.String)
		if err == nil {
			m.ForwardedFrom = &id
		}
	}
	if forwardedSenderName.Valid {
		m.ForwardedSenderName = &forwardedSenderName.String
		log.Printf("[DB] scanMessageWithEncryption: Set forwarded_sender_name=%s", forwardedSenderName.String)
	}
	if scheduledAt.Valid {
		m.ScheduledAt = &scheduledAt.Time
	}
	if threadID.Valid {
		id, err := uuid.Parse(threadID.String)
		if err == nil {
			m.ThreadID = &id
		}
	}
	return nil
}

const messageColumns = `m.id, m.chat_id, m.sender_id, m.type, m.content, m.file_url, m.created_at, m.updated_at, m.reply_to, m.encrypted_content, m.encrypted_keys, m.location, m.duration, m.forwarded_from, m.forwarded_sender_name, m.scheduled_at, m.thread_id, m.synced_to_xmpp, m.xmpp_message_id, reply_msg.content as reply_to_content, reply_sender.first_name || ' ' || reply_sender.last_name as reply_to_sender_name`

const messageColumnsBase = `m.id, m.chat_id, m.sender_id, m.type, m.content, m.file_url, m.created_at, m.updated_at, m.reply_to, m.encrypted_content, m.encrypted_keys, m.location, m.duration, m.forwarded_from, m.forwarded_sender_name, m.scheduled_at, m.thread_id, m.synced_to_xmpp, m.xmpp_message_id`

func (s *PostgresStorage) SearchMessages(ctx context.Context, chatID uuid.UUID, query string, senderID *uuid.UUID, dateFrom, dateTo *time.Time, limit, offset int) ([]*models.Message, error) {
	// Validate and sanitize query
	if len(query) > 200 {
		query = query[:200]
	}
	// Escape special ILIKE characters to prevent pattern abuse
	query = strings.ReplaceAll(query, "%", "\\%")
	query = strings.ReplaceAll(query, "_", "\\_")

	args := []interface{}{chatID}
	conditions := []string{"m.chat_id = $1"}

	if query != "" {
		args = append(args, "%"+query+"%")
		conditions = append(conditions, fmt.Sprintf("m.content ILIKE $%d", len(args)))
	}
	if senderID != nil {
		args = append(args, *senderID)
		conditions = append(conditions, fmt.Sprintf("m.sender_id = $%d", len(args)))
	}
	if dateFrom != nil {
		args = append(args, *dateFrom)
		conditions = append(conditions, fmt.Sprintf("m.created_at >= $%d", len(args)))
	}
	if dateTo != nil {
		args = append(args, *dateTo)
		conditions = append(conditions, fmt.Sprintf("m.created_at <= $%d", len(args)))
	}

	args = append(args, limit, offset)
	sqlQuery := `SELECT ` + messageColumns + `
		FROM messages m
		LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id
		LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY m.created_at DESC
		LIMIT $` + fmt.Sprintf("%d", len(args)-1) + ` OFFSET $` + fmt.Sprintf("%d", len(args))

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
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
	return messages, rows.Err()
}

func (s *PostgresStorage) UpdateMessage(ctx context.Context, msg *models.Message) error {
	query := `
		UPDATE messages 
		SET content = $1, updated_at = NOW() 
		WHERE id = $2
		RETURNING updated_at
	`
	return s.db.QueryRowContext(ctx, query, msg.Content, msg.ID).Scan(&msg.UpdatedAt)
}

func (s *PostgresStorage) UpdateMessageFileURL(ctx context.Context, messageID uuid.UUID, fileURL string) error {
	query := `
		UPDATE messages 
		SET file_url = $1, updated_at = NOW() 
		WHERE id = $2
	`
	_, err := s.db.ExecContext(ctx, query, fileURL, messageID)
	return err
}

func (s *PostgresStorage) DeleteMessage(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM messages WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

func (s *PostgresStorage) MarkMessageAsRead(ctx context.Context, messageID, userID uuid.UUID) error {
	query := `
		INSERT INTO message_reads (message_id, user_id, read_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (message_id, user_id) DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, query, messageID, userID)
	return err
}

func (s *PostgresStorage) MarkMessagesAsRead(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error {
	if len(messageIDs) == 0 {
		return nil
	}

	query := `
		INSERT INTO message_reads (message_id, user_id, read_at)
		SELECT unnest($1::uuid[]), $2, NOW()
		ON CONFLICT (message_id, user_id) DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, query, pq.Array(messageIDs), userID)
	return err
}

func (s *PostgresStorage) GetMessageReadStatus(ctx context.Context, messageID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT user_id FROM message_reads WHERE message_id = $1`
	rows, err := s.db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []uuid.UUID
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, rows.Err()
}

func (s *PostgresStorage) GetMessagesReadStatus(ctx context.Context, messageIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	if len(messageIDs) == 0 {
		return make(map[uuid.UUID][]uuid.UUID), nil
	}

	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, len(messageIDs))
	for i, id := range messageIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT message_id, user_id
		FROM message_reads
		WHERE message_id IN (%s)
	`, strings.Join(placeholders, ", "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]uuid.UUID)
	for rows.Next() {
		var messageID, userID uuid.UUID
		if err := rows.Scan(&messageID, &userID); err != nil {
			return nil, err
		}
		result[messageID] = append(result[messageID], userID)
	}
	return result, rows.Err()
}

func (s *PostgresStorage) PinMessage(ctx context.Context, chatID, messageID, userID uuid.UUID) error {
	query := `INSERT INTO pinned_messages (chat_id, message_id, user_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, chatID, messageID, userID)
	return err
}

func (s *PostgresStorage) UnpinMessage(ctx context.Context, chatID, messageID uuid.UUID) error {
	query := `DELETE FROM pinned_messages WHERE chat_id = $1 AND message_id = $2`
	_, err := s.db.ExecContext(ctx, query, chatID, messageID)
	return err
}

func (s *PostgresStorage) GetPinnedMessages(ctx context.Context, chatID uuid.UUID) ([]*models.Message, error) {
	query := `
		SELECT m.` + messageColumns + `
		FROM pinned_messages pm
		JOIN messages m ON pm.message_id = m.id
		WHERE pm.chat_id = $1
		ORDER BY pm.pinned_at DESC
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
	return messages, rows.Err()
}

func (s *PostgresStorage) GetFilesByMessage(ctx context.Context, messageID uuid.UUID) ([]*models.File, error) {
	query := `
		SELECT id, message_id, name, size, mime_type, url, uploaded_at
		FROM files
		WHERE message_id = $1
		ORDER BY uploaded_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*models.File
	for rows.Next() {
		file := &models.File{}
		if err := rows.Scan(&file.ID, &file.MessageID, &file.Name, &file.Size, &file.MimeType, &file.URL, &file.UploadedAt); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

// NotificationStorage implementation

func (s *PostgresStorage) SearchAllMessages(ctx context.Context, userID uuid.UUID, query string, senderID *uuid.UUID, dateFrom, dateTo *time.Time, limit, offset int) ([]*models.Message, error) {
	args := []interface{}{userID}
	conditions := []string{"cm.user_id = $1", "cm.archived = false", "cm.deleted_at IS NULL"}

	if query != "" {
		args = append(args, query)
		conditions = append(conditions, fmt.Sprintf("to_tsvector('russian', m.content) @@ plainto_tsquery('russian', $%d)", len(args)))
	}
	if senderID != nil {
		args = append(args, *senderID)
		conditions = append(conditions, fmt.Sprintf("m.sender_id = $%d", len(args)))
	}
	if dateFrom != nil {
		args = append(args, *dateFrom)
		conditions = append(conditions, fmt.Sprintf("m.created_at >= $%d", len(args)))
	}
	if dateTo != nil {
		args = append(args, *dateTo)
		conditions = append(conditions, fmt.Sprintf("m.created_at <= $%d", len(args)))
	}

	args = append(args, limit, offset)
	searchQuery := `SELECT ` + messageColumns + `
		FROM messages m
		JOIN chat_members cm ON m.chat_id = cm.chat_id
		LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id
		LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY m.created_at DESC
		LIMIT $` + fmt.Sprintf("%d", len(args)-1) + ` OFFSET $` + fmt.Sprintf("%d", len(args))

	rows, err := s.db.QueryContext(ctx, searchQuery, args...)
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
	return messages, nil
}

func (s *PostgresStorage) GetMessageReactions(ctx context.Context, messageID uuid.UUID) ([]*models.Reaction, error) {
	query := `
		SELECT id, message_id, user_id, emoji, created_at
		FROM reactions
		WHERE message_id = $1
		ORDER BY created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []*models.Reaction
	for rows.Next() {
		var r models.Reaction
		if err := rows.Scan(&r.ID, &r.MessageID, &r.UserID, &r.Emoji, &r.CreatedAt); err != nil {
			return nil, err
		}
		reactions = append(reactions, &r)
	}
	return reactions, nil
}

// XMPP sync methods
func (s *PostgresStorage) GetUnsyncedMessages(ctx context.Context, limit int) ([]*models.Message, error) {
	// Add time filter to prevent syncing millions of old messages
	// Only sync messages created in the last 30 days
	query := `
		SELECT ` + messageColumns + `
		FROM messages m
		LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id
		LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id
		WHERE (m.synced_to_xmpp = false OR m.synced_to_xmpp IS NULL)
		AND m.created_at > NOW() - INTERVAL '30 days'
		ORDER BY m.created_at ASC
		LIMIT $1
	`
	rows, err := s.db.QueryContext(ctx, query, limit)
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
	return messages, rows.Err()
}

func (s *PostgresStorage) MarkMessageAsSyncedToXMPP(ctx context.Context, messageID uuid.UUID, xmppMessageID string) error {
	query := `
		UPDATE messages
		SET synced_to_xmpp = true, xmpp_message_id = $1
		WHERE id = $2
	`
	_, err := s.db.ExecContext(ctx, query, xmppMessageID, messageID)
	return err
}

func (s *PostgresStorage) GetMessageByXMPPID(ctx context.Context, xmppMessageID string) (*models.Message, error) {
	msg := &models.Message{}
	query := `SELECT ` + messageColumns + ` FROM messages m LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id WHERE m.xmpp_message_id = $1`
	err := scanMessageWithEncryption(
		s.db.QueryRowContext(ctx, query, xmppMessageID),
		msg,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return msg, nil
}

func (s *PostgresStorage) GetScheduledMessages(ctx context.Context) ([]*models.Message, error) {
	query := `
		SELECT ` + messageColumns + `
		FROM messages m
		LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id
		LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id
		WHERE m.scheduled_at IS NOT NULL AND m.scheduled_at <= NOW()
		ORDER BY m.scheduled_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
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
	return messages, rows.Err()
}

func (s *PostgresStorage) GetThreadMessages(ctx context.Context, threadID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	query := `
		SELECT ` + messageColumns + `
		FROM messages m
		LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id
		LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id
		WHERE m.thread_id = $1
		ORDER BY m.created_at ASC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.db.QueryContext(ctx, query, threadID, limit, offset)
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
	return messages, rows.Err()
}
