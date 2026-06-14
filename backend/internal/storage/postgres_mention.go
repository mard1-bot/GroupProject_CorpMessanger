package storage

import (
	"context"

	"github.com/google/uuid"

	"corp-messenger/backend/internal/models"
)

func (s *PostgresStorage) AddMention(ctx context.Context, messageID, mentionedUserID uuid.UUID) error {
	query := `INSERT INTO mentions (message_id, mentioned_user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, messageID, mentionedUserID)
	return err
}

func (s *PostgresStorage) GetUserMentions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Mention, error) {
	query := `
		SELECT id, message_id, mentioned_user_id, created_at
		FROM mentions
		WHERE mentioned_user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mentions []*models.Mention
	for rows.Next() {
		var m models.Mention
		if err := rows.Scan(&m.ID, &m.MessageID, &m.MentionedUserID, &m.CreatedAt); err != nil {
			return nil, err
		}
		mentions = append(mentions, &m)
	}
	return mentions, nil
}
