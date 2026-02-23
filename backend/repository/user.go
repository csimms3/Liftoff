package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"liftoff/backend/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepository manages user-related database operations
type UserRepository struct {
	db        *pgxpool.Pool
	sqlite    *sql.DB
	useSQLite bool
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *pgxpool.Pool, sqlite *sql.DB, useSQLite bool) *UserRepository {
	if useSQLite {
		return &UserRepository{db: nil, sqlite: sqlite, useSQLite: true}
	}
	return &UserRepository{db: db, sqlite: nil, useSQLite: false}
}

// CreateUser creates a new user with hashed password
func (r *UserRepository) CreateUser(ctx context.Context, email, passwordHash string) (*models.User, error) {
	id := uuid.New().String()

	if r.useSQLite {
		return r.createUserSQLite(ctx, id, email, passwordHash)
	}
	return r.createUserPostgres(ctx, id, email, passwordHash)
}

func (r *UserRepository) createUserPostgres(ctx context.Context, id, email, passwordHash string) (*models.User, error) {
	query := `
		INSERT INTO users (id, email, password_hash, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id, email, created_at
	`

	var user models.User
	err := r.db.QueryRow(ctx, query, id, email, passwordHash).Scan(
		&user.ID, &user.Email, &user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &user, nil
}

func (r *UserRepository) createUserSQLite(ctx context.Context, id, email, passwordHash string) (*models.User, error) {
	query := `
		INSERT INTO users (id, email, password_hash, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`

	_, err := r.sqlite.ExecContext(ctx, query, id, email, passwordHash)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	var user models.User
	err = r.sqlite.QueryRowContext(ctx, "SELECT id, email, created_at FROM users WHERE id = ?", id).Scan(
		&user.ID, &user.Email, &user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created user: %w", err)
	}

	return &user, nil
}

// CreatePasswordResetToken creates a reset token for the user
func (r *UserRepository) CreatePasswordResetToken(ctx context.Context, userID string, tokenHash string, expiresAt time.Time) error {
	id := uuid.New().String()
	if r.useSQLite {
		return r.createPasswordResetTokenSQLite(ctx, id, userID, tokenHash, expiresAt)
	}
	return r.createPasswordResetTokenPostgres(ctx, id, userID, tokenHash, expiresAt)
}

func (r *UserRepository) createPasswordResetTokenPostgres(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, id, userID, tokenHash, expiresAt)
	return err
}

func (r *UserRepository) createPasswordResetTokenSQLite(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.sqlite.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, id, userID, tokenHash, expiresAt)
	return err
}

// GetUserIDByResetToken returns user ID if token is valid and not expired
func (r *UserRepository) GetUserIDByResetToken(ctx context.Context, tokenHash string) (string, error) {
	if r.useSQLite {
		return r.getUserIDByResetTokenSQLite(ctx, tokenHash)
	}
	return r.getUserIDByResetTokenPostgres(ctx, tokenHash)
}

func (r *UserRepository) getUserIDByResetTokenPostgres(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := r.db.QueryRow(ctx, `
		SELECT user_id FROM password_reset_tokens
		WHERE token_hash = $1 AND expires_at > NOW()
		LIMIT 1
	`, tokenHash).Scan(&userID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return userID, err
}

func (r *UserRepository) getUserIDByResetTokenSQLite(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := r.sqlite.QueryRowContext(ctx, `
		SELECT user_id FROM password_reset_tokens
		WHERE token_hash = ? AND expires_at > datetime('now')
		LIMIT 1
	`, tokenHash).Scan(&userID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return userID, err
}

// DeletePasswordResetToken removes used/expired tokens for a user
func (r *UserRepository) DeletePasswordResetToken(ctx context.Context, tokenHash string) error {
	if r.useSQLite {
		_, err := r.sqlite.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE token_hash = ?`, tokenHash)
		return err
	}
	_, err := r.db.Exec(ctx, `DELETE FROM password_reset_tokens WHERE token_hash = $1`, tokenHash)
	return err
}

// UpdatePassword updates a user's password
func (r *UserRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	if r.useSQLite {
		_, err := r.sqlite.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
		return err
	}
	_, err := r.db.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, passwordHash, userID)
	return err
}

// GenerateSecureToken creates a cryptographically secure random token
func GenerateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GetByEmail retrieves a user by email (case-insensitive)
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if r.useSQLite {
		return r.getByEmailSQLite(ctx, email)
	}
	return r.getByEmailPostgres(ctx, email)
}

func (r *UserRepository) getByEmailPostgres(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, created_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`

	var user models.User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (r *UserRepository) getByEmailSQLite(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, created_at
		FROM users
		WHERE LOWER(email) = LOWER(?)
	`

	var user models.User
	err := r.sqlite.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	if r.useSQLite {
		return r.getByIDSQLite(ctx, id)
	}
	return r.getByIDPostgres(ctx, id)
}

func (r *UserRepository) getByIDPostgres(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, email, created_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := r.db.QueryRow(ctx, query, id).Scan(&user.ID, &user.Email, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (r *UserRepository) getByIDSQLite(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, email, created_at
		FROM users
		WHERE id = ?
	`

	var user models.User
	err := r.sqlite.QueryRowContext(ctx, query, id).Scan(&user.ID, &user.Email, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}
