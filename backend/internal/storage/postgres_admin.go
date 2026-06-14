package storage

import (
	"context"

	"corp-messenger/backend/internal/models"
)

// GetAdminStats returns system statistics for admin dashboard
func (s *PostgresStorage) GetAdminStats(ctx context.Context) (*models.AdminStats, error) {
	stats := &models.AdminStats{}

	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers)
	if err != nil {
		return nil, err
	}

	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE status = 'active'`).Scan(&stats.ActiveUsers)
	if err != nil {
		return nil, err
	}

	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE status = 'blocked'`).Scan(&stats.BlockedUsers)
	if err != nil {
		return nil, err
	}

	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&stats.AdminCount)
	if err != nil {
		return nil, err
	}

	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'moderator'`).Scan(&stats.ModeratorCount)
	if err != nil {
		return nil, err
	}

	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chats`).Scan(&stats.TotalChats)
	if err != nil {
		return nil, err
	}

	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages`).Scan(&stats.TotalMessages)
	if err != nil {
		return nil, err
	}

	// Users with last_online within last 5 minutes
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE last_online > NOW() - INTERVAL '5 minutes'`).Scan(&stats.OnlineUsers)
	if err != nil {
		// last_online might not exist, default to 0
		stats.OnlineUsers = 0
	}

	return stats, nil
}
