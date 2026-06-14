package storage

import (
	"context"

	"github.com/google/uuid"
)

func (s *PostgresStorage) AddReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	query := `INSERT INTO reactions (message_id, user_id, emoji) VALUES ($1, $2, $3) ON CONFLICT (message_id, user_id, emoji) DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, messageID, userID, emoji)
	return err
}

func (s *PostgresStorage) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	query := `DELETE FROM reactions WHERE message_id = $1 AND user_id = $2 AND emoji = $3`
	_, err := s.db.ExecContext(ctx, query, messageID, userID, emoji)
	return err
}
