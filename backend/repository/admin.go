package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminStats holds aggregate statistics for the admin panel
type AdminStats struct {
	TotalUsers    int `json:"total_users"`
	TotalWorkouts int `json:"total_workouts"`
	TotalSessions int `json:"total_sessions"`
	NewUsers7d    int `json:"new_users_7d"`
}

// AdminRepository provides admin-only data access
type AdminRepository struct {
	db *pgxpool.Pool
}

// NewAdminRepository creates a new admin repository
func NewAdminRepository(db *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{db: db}
}

// GetStats returns aggregate statistics
func (r *AdminRepository) GetStats(ctx context.Context) (*AdminStats, error) {
	s := &AdminStats{}
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&s.TotalUsers)
	if err != nil {
		return nil, err
	}
	err = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM workouts`).Scan(&s.TotalWorkouts)
	if err != nil {
		return nil, err
	}
	err = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM workout_sessions`).Scan(&s.TotalSessions)
	if err != nil {
		return nil, err
	}
	err = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE created_at > NOW() - INTERVAL '7 days'`).Scan(&s.NewUsers7d)
	if err != nil {
		return nil, err
	}
	return s, nil
}
