package storage

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"corp-messenger/backend/internal/models"
)

func (s *PostgresStorage) CreateFile(ctx context.Context, file *models.File) error {
	query := `
		INSERT INTO files (id, message_id, name, size, mime_type, url, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := s.db.ExecContext(ctx, query,
		file.ID, file.MessageID, file.Name, file.Size, file.MimeType, file.URL, file.UploadedAt,
	)
	return err
}

func (s *PostgresStorage) GetFileByID(ctx context.Context, id uuid.UUID) (*models.File, error) {
	file := &models.File{}
	query := `
		SELECT id, message_id, name, size, mime_type, url, uploaded_at
		FROM files
		WHERE id = $1
	`
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&file.ID, &file.MessageID, &file.Name, &file.Size, &file.MimeType, &file.URL, &file.UploadedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return file, nil
}
