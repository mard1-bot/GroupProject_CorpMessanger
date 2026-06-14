package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"time"

	"github.com/google/uuid"

	"corp-messenger/backend/internal/models"
)

func (s *PostgresStorage) GetNotificationSettings(ctx context.Context, userID uuid.UUID) (*models.NotificationSettings, error) {
	settings := &models.NotificationSettings{UserID: userID}
	query := `
		SELECT push_enabled, email_enabled, email,
		       quiet_hours_start, quiet_hours_end,
		       quiet_hours_enabled, protocol_preference, hybrid_mode_enabled, updated_at
		FROM notification_settings
		WHERE user_id = $1
	`
	var quietStartStr, quietEndStr sql.NullString
	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&settings.PushEnabled, &settings.EmailEnabled, &settings.Email,
		&quietStartStr, &quietEndStr, &settings.QuietHoursEnabled, &settings.ProtocolPreference, &settings.HybridModeEnabled, &settings.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Return default settings
			return &models.NotificationSettings{
				UserID:             userID,
				PushEnabled:        true,
				EmailEnabled:       true,
				HybridModeEnabled:  false,
				ProtocolPreference: "websocket",
				UpdatedAt:          time.Now(),
			}, nil
		}
		return nil, err
	}
	// Set quiet hours strings if they exist
	if quietStartStr.Valid {
		settings.QuietHoursStart = &quietStartStr.String
	}
	if quietEndStr.Valid {
		settings.QuietHoursEnd = &quietEndStr.String
	}
	return settings, nil
}

func (s *PostgresStorage) UpdateNotificationSettings(ctx context.Context, settings *models.NotificationSettings) error {
	// Convert string times to proper time.Time for database
	var quietStart, quietEnd interface{}
	if settings.QuietHoursStart != nil {
		if t, err := time.Parse("15:04", *settings.QuietHoursStart); err == nil {
			quietStart = t
		} else {
			quietStart = nil
		}
	}
	if settings.QuietHoursEnd != nil {
		if t, err := time.Parse("15:04", *settings.QuietHoursEnd); err == nil {
			quietEnd = t
		} else {
			quietEnd = nil
		}
	}

	// Ensure protocol_preference has a valid value
	protocolPreference := settings.ProtocolPreference
	if protocolPreference == "" {
		protocolPreference = "websocket"
	}

	// Log the values being sent to database
	fmt.Printf("[PostgresStorage] UpdateNotificationSettings: protocol_preference=%q, settings.ProtocolPreference=%q, quiet_hours_enabled=%v\n", protocolPreference, settings.ProtocolPreference, settings.QuietHoursEnabled)

	query := `
		INSERT INTO notification_settings (user_id, push_enabled, email_enabled, email, quiet_hours_start, quiet_hours_end, quiet_hours_enabled, protocol_preference, hybrid_mode_enabled, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			push_enabled = EXCLUDED.push_enabled,
			email_enabled = EXCLUDED.email_enabled,
			email = COALESCE(EXCLUDED.email, notification_settings.email),
			quiet_hours_start = EXCLUDED.quiet_hours_start,
			quiet_hours_end = EXCLUDED.quiet_hours_end,
			quiet_hours_enabled = EXCLUDED.quiet_hours_enabled,
			protocol_preference = CASE WHEN EXCLUDED.protocol_preference IS NULL OR EXCLUDED.protocol_preference = '' THEN 'websocket' ELSE EXCLUDED.protocol_preference END,
			hybrid_mode_enabled = EXCLUDED.hybrid_mode_enabled,
			updated_at = EXCLUDED.updated_at
		RETURNING updated_at
	`
	return s.db.QueryRowContext(ctx, query,
		settings.UserID, settings.PushEnabled, settings.EmailEnabled, settings.Email,
		quietStart, quietEnd, settings.QuietHoursEnabled,
		protocolPreference, settings.HybridModeEnabled,
	).Scan(&settings.UpdatedAt)
}
