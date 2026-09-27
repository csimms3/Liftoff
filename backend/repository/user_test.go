package repository_test

import (
	"context"
	"testing"

	"liftoff/backend/internal/testdb"
	"liftoff/backend/repository"
)

// userRepo returns a UserRepository on a fresh schema and a raw-SQL helper.
func userRepo(t *testing.T) (*repository.UserRepository, func(string)) {
	pool := testdb.Postgres(t)
	return repository.NewUserRepository(pool), func(q string) { testdb.Exec(t, pool, q) }
}

func TestUserRepository_UnknownUserIsNilNotError(t *testing.T) {
	repo, _ := userRepo(t)
	ctx := context.Background()

	u, err := repo.GetByEmail(ctx, "nobody@example.com")
	if err != nil || u != nil {
		t.Errorf("GetByEmail(unknown) = %v, %v; want nil, nil", u, err)
	}
	u, err = repo.GetByID(ctx, "no-such-id")
	if err != nil || u != nil {
		t.Errorf("GetByID(unknown) = %v, %v; want nil, nil", u, err)
	}
	id, err := repo.GetUserIDByResetToken(ctx, "no-such-token")
	if err != nil || id != "" {
		t.Errorf("GetUserIDByResetToken(unknown) = %q, %v; want empty, nil", id, err)
	}
}

func TestUserRepository_IsAdmin(t *testing.T) {
	repo, exec := userRepo(t)
	ctx := context.Background()

	u, err := repo.CreateUser(ctx, "me@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.IsAdmin(ctx, u.ID); err != nil || ok {
		t.Errorf("new user IsAdmin = %v, %v; want false", ok, err)
	}

	exec(`UPDATE users SET is_admin = true WHERE email = 'me@example.com'`)
	if ok, err := repo.IsAdmin(ctx, u.ID); err != nil || !ok {
		t.Errorf("flagged user IsAdmin = %v, %v; want true", ok, err)
	}
	got, err := repo.GetByEmail(ctx, "ME@example.com")
	if err != nil || got == nil || !got.IsAdmin {
		t.Errorf("GetByEmail should carry is_admin: %+v, %v", got, err)
	}
	if ok, err := repo.IsAdmin(ctx, "no-such-id"); err != nil || ok {
		t.Errorf("unknown user IsAdmin = %v, %v; want false", ok, err)
	}
}
