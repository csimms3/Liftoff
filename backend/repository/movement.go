package repository

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// rowQuerier is satisfied by both the pool and a pgx transaction.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// MaxExerciseNameLength is the movement/exercise name column size.
const MaxExerciseNameLength = 255

// ValidExerciseName reports whether name is non-blank and fits the column.
func ValidExerciseName(name string) bool {
	n := NormalizeExerciseName(name)
	return n != "" && utf8.RuneCountInString(n) <= MaxExerciseNameLength
}

// NormalizeExerciseName trims surrounding spaces, the same rule as the
// movements index (BTRIM), so Go and SQL agree on which names match.
func NormalizeExerciseName(name string) string {
	return strings.Trim(name, " ")
}

// findOrCreateMovement returns the id of userID's movement called name (matched
// ignoring case and surrounding spaces), creating it if needed. Safe under
// concurrent calls: the unique index decides.
func findOrCreateMovement(ctx context.Context, q rowQuerier, userID, name string) (string, error) {
	name = NormalizeExerciseName(name)
	if name == "" {
		return "", fmt.Errorf("exercise name is empty")
	}
	now := time.Now()
	var id string
	err := q.QueryRow(ctx, `
		INSERT INTO movements (id, user_id, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (user_id, (LOWER(BTRIM(name)))) DO UPDATE SET name = movements.name
		RETURNING id`, uuid.New().String(), userID, name, now).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("find or create movement: %w", err)
	}
	return id, nil
}
