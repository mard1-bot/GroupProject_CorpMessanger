package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"corp-messenger/backend/internal/models"
)

func (s *PostgresStorage) CreateUser(ctx context.Context, user *models.User, passwordHash string) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// Handle rollback with error check
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("transaction error: %w; rollback failed: %v", err, rbErr)
			}
		}
	}()

	query := `
		INSERT INTO users (email, phone, first_name, last_name, middle_name, avatar, status, role)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`
	var middleName, avatar sql.NullString
	if user.MiddleName != nil {
		middleName.String = *user.MiddleName
		middleName.Valid = true
	}
	if user.Avatar != nil {
		avatar.String = *user.Avatar
		avatar.Valid = true
	}
	err = tx.QueryRowContext(ctx, query,
		user.Email, user.Phone, user.FirstName, user.LastName, middleName,
		avatar, user.Status, user.Role,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	credQuery := `INSERT INTO user_credentials (user_id, password_hash) VALUES ($1, $2)`
	_, err = tx.ExecContext(ctx, credQuery, user.ID, passwordHash)
	if err != nil {
		return fmt.Errorf("insert credentials: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *PostgresStorage) UpdateUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := `UPDATE user_credentials SET password_hash = $1 WHERE user_id = $2`
	_, err := s.db.ExecContext(ctx, query, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

func (s *PostgresStorage) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// Handle rollback with error check
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("transaction error: %w; rollback failed: %v", err, rbErr)
			}
		}
	}()

	// Delete user credentials
	_, err = tx.ExecContext(ctx, `DELETE FROM user_credentials WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete credentials: %w", err)
	}

	// Delete user
	_, err = tx.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *PostgresStorage) GetUserByEmail(ctx context.Context, email string) (*models.User, string, error) {
	user := &models.User{}
	var passwordHash string
	var middleName, phone, avatar sql.NullString

	query := `
		SELECT u.id, u.email, u.username, u.phone, u.first_name, u.last_name, u.middle_name,
		       u.avatar, u.status, u.role, u.created_at, u.updated_at, u.last_online, uc.password_hash
		FROM users u
		JOIN user_credentials uc ON u.id = uc.user_id
		WHERE u.email = $1
	`
	var username sql.NullString
	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &username, &phone, &user.FirstName, &user.LastName,
		&middleName, &avatar, &user.Status, &user.Role,
		&user.CreatedAt, &user.UpdatedAt, &user.LastOnline, &passwordHash,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", nil
		}
		return nil, "", err
	}

	if username.Valid {
		user.Username = &username.String
	}
	if middleName.Valid {
		user.MiddleName = &middleName.String
	}
	if phone.Valid {
		user.Phone = phone.String
	}
	if avatar.Valid {
		user.Avatar = &avatar.String
	}

	return user, passwordHash, nil
}

func (s *PostgresStorage) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user := &models.User{}
	var middleName, phone, avatar sql.NullString

	query := `
		SELECT id, email, username, phone, first_name, last_name, middle_name,
		       avatar, status, role, created_at, updated_at, last_online
		FROM users WHERE id = $1
	`
	var username sql.NullString
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &username, &phone, &user.FirstName, &user.LastName,
		&middleName, &avatar, &user.Status, &user.Role,
		&user.CreatedAt, &user.UpdatedAt, &user.LastOnline,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if username.Valid {
		user.Username = &username.String
	}
	if middleName.Valid {
		user.MiddleName = &middleName.String
	}
	if phone.Valid {
		user.Phone = phone.String
	}
	if avatar.Valid {
		user.Avatar = &avatar.String
	}

	return user, nil
}

func (s *PostgresStorage) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	user := &models.User{}
	var middleName, phone, avatar, uname sql.NullString

	query := `
		SELECT id, email, username, phone, first_name, last_name, middle_name,
		       avatar, status, role, created_at, updated_at, last_online
		FROM users WHERE LOWER(username) = LOWER($1)
	`
	err := s.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Email, &uname, &phone, &user.FirstName, &user.LastName,
		&middleName, &avatar, &user.Status, &user.Role,
		&user.CreatedAt, &user.UpdatedAt, &user.LastOnline,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if uname.Valid {
		user.Username = &uname.String
	}
	if middleName.Valid {
		user.MiddleName = &middleName.String
	}
	if phone.Valid {
		user.Phone = phone.String
	}
	if avatar.Valid {
		user.Avatar = &avatar.String
	}

	return user, nil
}

func (s *PostgresStorage) GetUserByIDWithPassword(ctx context.Context, id uuid.UUID) (*models.User, string, error) {
	user := &models.User{}
	var middleName, phone, avatar sql.NullString
	var passwordHash sql.NullString

	query := `
		SELECT u.id, u.email, u.username, u.phone, u.first_name, u.last_name, u.middle_name,
		       u.avatar, u.status, u.role, u.created_at, u.updated_at, u.last_online,
		       p.password_hash
		FROM users u
		LEFT JOIN user_credentials p ON u.id = p.user_id
		WHERE u.id = $1
	`
	var username sql.NullString
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &username, &phone, &user.FirstName, &user.LastName,
		&middleName, &avatar, &user.Status, &user.Role,
		&user.CreatedAt, &user.UpdatedAt, &user.LastOnline,
		&passwordHash,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", nil
		}
		return nil, "", err
	}

	if username.Valid {
		user.Username = &username.String
	}
	if middleName.Valid {
		user.MiddleName = &middleName.String
	}
	if phone.Valid {
		user.Phone = phone.String
	}
	if avatar.Valid {
		user.Avatar = &avatar.String
	}

	if passwordHash.Valid {
		return user, passwordHash.String, nil
	}
	return user, "", nil
}

func (s *PostgresStorage) UpdateUser(ctx context.Context, user *models.User) error {
	// Security: Only allow updating safe profile fields
	// Email, status, and role require separate admin endpoints
	query := `
		UPDATE users SET
			username = $1, phone = $2, first_name = $3, last_name = $4,
			middle_name = $5, avatar = $6,
			updated_at = NOW()
		WHERE id = $7
		RETURNING updated_at
	`
	var username, middleName, avatar sql.NullString
	if user.Username != nil {
		username.String = *user.Username
		username.Valid = true
	}
	if user.MiddleName != nil {
		middleName.String = *user.MiddleName
		middleName.Valid = true
	}
	if user.Avatar != nil {
		avatar.String = *user.Avatar
		avatar.Valid = true
	}
	err := s.db.QueryRowContext(ctx, query,
		username, user.Phone, user.FirstName, user.LastName,
		middleName, avatar, user.ID,
	).Scan(&user.UpdatedAt)
	return err
}

func (s *PostgresStorage) UpdateUserStatus(ctx context.Context, userID uuid.UUID, status, customStatus string) error {
	query := `
		UPDATE users SET
			status = $1, custom_status = $2, updated_at = NOW()
		WHERE id = $3
	`
	_, err := s.db.ExecContext(ctx, query, status, customStatus, userID)
	return err
}

func (s *PostgresStorage) GetUsers(ctx context.Context, search string, excludeUserID string, limit int) ([]*models.User, error) {
	// Default limit to prevent unbounded results
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Build query dynamically based on search
	var args []interface{}
	argIdx := 1

	whereClause := "WHERE status = 'active'"

	// Exclude current user if specified
	// SECURITY: Column name 'id' is hardcoded, only parameter index is dynamic ($%d)
	// This is safe from SQL injection as column names are not user-controlled
	if excludeUserID != "" {
		whereClause += fmt.Sprintf(" AND id != $%d", argIdx)
		args = append(args, excludeUserID)
		argIdx++
	}

	// Add search filter (email, first_name, last_name, username)
	if search != "" {
		searchPattern := "%" + strings.ToLower(search) + "%"
		whereClause += fmt.Sprintf(" AND (LOWER(email) LIKE $%d OR LOWER(first_name) LIKE $%d OR LOWER(last_name) LIKE $%d OR LOWER(username) LIKE $%d)", argIdx, argIdx+1, argIdx+2, argIdx+3)
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern)
		argIdx += 4
	}

	// Add limit
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT id, email, username, phone, first_name, last_name, middle_name,
		       avatar, status, role, created_at, updated_at, last_online
		FROM users %s
		ORDER BY created_at DESC
		LIMIT $%d
	`, whereClause, argIdx)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]*models.User, 0)
	for rows.Next() {
		user := &models.User{}
		var username, middleName, phone, avatar sql.NullString
		err := rows.Scan(
			&user.ID, &user.Email, &username, &phone, &user.FirstName, &user.LastName,
			&middleName, &avatar, &user.Status, &user.Role,
			&user.CreatedAt, &user.UpdatedAt, &user.LastOnline,
		)
		if err != nil {
			return nil, err
		}

		if username.Valid {
			user.Username = &username.String
		}
		if middleName.Valid {
			user.MiddleName = &middleName.String
		}
		if phone.Valid {
			user.Phone = phone.String
		}
		if avatar.Valid {
			user.Avatar = &avatar.String
		}

		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *PostgresStorage) GetUserLastOnline(ctx context.Context, userID uuid.UUID) (time.Time, error) {
	// Use last session activity or WebSocket connection
	query := `
		SELECT COALESCE(MAX(last_used_at), '1970-01-01'::timestamp)
		FROM device_tokens
		WHERE user_id = $1
	`
	var lastOnline time.Time
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&lastOnline)
	return lastOnline, err
}

func (s *PostgresStorage) UpdateUserLastOnline(ctx context.Context, userID uuid.UUID) error {
	// Update device token timestamp
	_, err := s.db.ExecContext(ctx, `
		UPDATE device_tokens SET last_used_at = NOW() WHERE user_id = $1
	`, userID)
	return err
}

// VerifyUserPassword verifies a user's password
func (s *PostgresStorage) VerifyUserPassword(ctx context.Context, userID uuid.UUID, password string) error {
	query := `
		SELECT password_hash
		FROM user_credentials
		WHERE user_id = $1
	`
	var passwordHash string
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&passwordHash)
	if err != nil {
		return err
	}
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
}

// UpdateUserRole updates a user's role
func (s *PostgresStorage) UpdateUserRole(ctx context.Context, userID uuid.UUID, role string) error {
	query := `UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2`
	_, err := s.db.ExecContext(ctx, query, role, userID)
	return err
}
