package repository

import (
	"context"
	"database/sql"
	"fmt"

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
