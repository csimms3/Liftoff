package repository_test

import (
	"context"
	"testing"

	"liftoff/backend/internal/testdb"
	"liftoff/backend/repository"
)

// userRepos returns a UserRepository per backend; Postgres is skipped when unconfigured.
func userRepos(t *testing.T) map[string]func(t *testing.T) (*repository.UserRepository, func(string)) {
	return map[string]func(t *testing.T) (*repository.UserRepository, func(string)){
		"sqlite": func(t *testing.T) (*repository.UserRepository, func(string)) {
			db := testdb.SQLite(t)
			return repository.NewUserRepository(nil, db.GetSQLite(), true), func(q string) {
				if _, err := db.GetSQLite().Exec(q); err != nil {
					t.Fatal(err)
				}
			}
		},
		"postgres": func(t *testing.T) (*repository.UserRepository, func(string)) {
			pool := testdb.Postgres(t)
			return repository.NewUserRepository(pool, nil, false), func(q string) { testdb.Exec(t, pool, q) }
		},
	}
}

func TestUserRepository_UnknownUserIsNilNotError(t *testing.T) {
	for name, open := range userRepos(t) {
		t.Run(name, func(t *testing.T) {
			repo, _ := open(t)
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
		})
	}
}

func TestUserRepository_IsAdmin(t *testing.T) {
	for name, open := range userRepos(t) {
		t.Run(name, func(t *testing.T) {
			repo, exec := open(t)
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
		})
	}
}
