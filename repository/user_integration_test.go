//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"connection_manager/domain"
)

var (
	t0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t1 = t0.Add(time.Hour)
)

func newTestUser(t *testing.T, email string, now time.Time) domain.User {
	t.Helper()
	u, err := domain.NewUser(email, "hash-"+email, now)
	require.NoError(t, err)
	return u
}

func requireSameUser(t *testing.T, want, got domain.User) {
	t.Helper()
	require.Equal(t, want.ID, got.ID)
	require.Equal(t, want.Email, got.Email)
	require.Equal(t, want.PasswordHash, got.PasswordHash)
	require.True(t, want.CreatedAt.Equal(got.CreatedAt), "created_at: want %v got %v", want.CreatedAt, got.CreatedAt)
	require.True(t, want.UpdatedAt.Equal(got.UpdatedAt), "updated_at: want %v got %v", want.UpdatedAt, got.UpdatedAt)
	require.Equal(t, time.UTC, got.CreatedAt.Location())
	require.Equal(t, time.UTC, got.UpdatedAt.Location())
}

func TestUserRepository_CreateAndGet(t *testing.T) {
	truncate(t, "users")
	ctx := context.Background()
	repo := NewUserRepository(testPool)

	u := newTestUser(t, "alice@example.com", t0)
	require.NoError(t, repo.Create(ctx, u))

	got, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	requireSameUser(t, u, got)
}

func TestUserRepository_Create_DuplicateEmail(t *testing.T) {
	truncate(t, "users")
	ctx := context.Background()
	repo := NewUserRepository(testPool)

	require.NoError(t, repo.Create(ctx, newTestUser(t, "alice@example.com", t0)))
	err := repo.Create(ctx, newTestUser(t, "alice@example.com", t1))
	require.ErrorIs(t, err, domain.ErrEmailTaken)
	require.ErrorIs(t, err, domain.ErrConflict)
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	truncate(t, "users")
	_, err := NewUserRepository(testPool).GetByID(context.Background(), uuid.Must(uuid.NewV7()))
	require.ErrorIs(t, err, domain.ErrUserNotFound)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUserRepository_List(t *testing.T) {
	ctx := context.Background()
	repo := NewUserRepository(testPool)

	t.Run("empty", func(t *testing.T) {
		truncate(t, "users")
		users, err := repo.List(ctx)
		require.NoError(t, err)
		require.NotNil(t, users)
		require.Empty(t, users)
	})

	t.Run("ordered by created_at", func(t *testing.T) {
		truncate(t, "users")
		second := newTestUser(t, "bob@example.com", t1)
		first := newTestUser(t, "alice@example.com", t0)
		require.NoError(t, repo.Create(ctx, second))
		require.NoError(t, repo.Create(ctx, first))

		users, err := repo.List(ctx)
		require.NoError(t, err)
		require.Len(t, users, 2)
		requireSameUser(t, first, users[0])
		requireSameUser(t, second, users[1])
	})
}

func TestUserRepository_Update(t *testing.T) {
	ctx := context.Background()
	repo := NewUserRepository(testPool)

	t.Run("persists changes", func(t *testing.T) {
		truncate(t, "users")
		u := newTestUser(t, "alice@example.com", t0)
		require.NoError(t, repo.Create(ctx, u))

		require.NoError(t, u.ChangeEmail("alice2@example.com", t1))
		require.NoError(t, u.ChangePassword("new-hash", t1))
		require.NoError(t, repo.Update(ctx, u))

		got, err := repo.GetByID(ctx, u.ID)
		require.NoError(t, err)
		requireSameUser(t, u, got)
	})

	t.Run("not found", func(t *testing.T) {
		truncate(t, "users")
		err := repo.Update(ctx, newTestUser(t, "ghost@example.com", t0))
		require.ErrorIs(t, err, domain.ErrUserNotFound)
	})

	t.Run("duplicate email", func(t *testing.T) {
		truncate(t, "users")
		alice := newTestUser(t, "alice@example.com", t0)
		bob := newTestUser(t, "bob@example.com", t0)
		require.NoError(t, repo.Create(ctx, alice))
		require.NoError(t, repo.Create(ctx, bob))

		require.NoError(t, bob.ChangeEmail("alice@example.com", t1))
		err := repo.Update(ctx, bob)
		require.ErrorIs(t, err, domain.ErrEmailTaken)
	})
}

func TestUserRepository_Delete(t *testing.T) {
	ctx := context.Background()
	repo := NewUserRepository(testPool)

	t.Run("deletes", func(t *testing.T) {
		truncate(t, "users")
		u := newTestUser(t, "alice@example.com", t0)
		require.NoError(t, repo.Create(ctx, u))

		require.NoError(t, repo.Delete(ctx, u.ID))
		_, err := repo.GetByID(ctx, u.ID)
		require.ErrorIs(t, err, domain.ErrUserNotFound)
	})

	t.Run("not found", func(t *testing.T) {
		truncate(t, "users")
		err := repo.Delete(ctx, uuid.Must(uuid.NewV7()))
		require.ErrorIs(t, err, domain.ErrUserNotFound)
	})
}
