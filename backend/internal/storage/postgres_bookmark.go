package storage

import (
	"context"

	"github.com/google/uuid"

	"corp-messenger/backend/internal/models"
)

func (s *PostgresStorage) AddBookmark(ctx context.Context, userID, messageID uuid.UUID) error {
	query := `INSERT INTO bookmarks (user_id, message_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, userID, messageID)
	return err
}

func (s *PostgresStorage) RemoveBookmark(ctx context.Context, userID, messageID uuid.UUID) error {
	query := `DELETE FROM bookmarks WHERE user_id = $1 AND message_id = $2`
	_, err := s.db.ExecContext(ctx, query, userID, messageID)
	return err
}

func (s *PostgresStorage) GetBookmarks(ctx context.Context, userID uuid.UUID) ([]*models.Message, error) {
	query := `SELECT ` + messageColumns + ` FROM messages m
		JOIN bookmarks b ON b.message_id = m.id
		LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id
		LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id
		WHERE b.user_id = $1
		ORDER BY b.created_at DESC`
	rows, err := s.db.QueryContext(ctx, query, userID)
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
