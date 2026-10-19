package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	t0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t1 = t0.Add(time.Hour)
)

func TestNewUser(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		hash      string
		wantEmail string
		wantErr   error
	}{
		{"valid", "alice@example.com", "h", "alice@example.com", nil},
		{"trims and lowercases", "  Alice@Example.COM \n", "h", "alice@example.com", nil},
		{"empty email", "", "h", "", ErrValidation},
		{"blank email", "   ", "h", "", ErrValidation},
		{"no at sign", "alice.example.com", "h", "", ErrValidation},
		{"no local part", "@example.com", "h", "", ErrValidation},
		{"display name form", "Alice <alice@example.com>", "h", "", ErrValidation},
		{"angle brackets only", "<alice@example.com>", "h", "", ErrValidation},
		{"two addresses", "a@example.com, b@example.com", "h", "", ErrValidation},
		{"empty hash", "alice@example.com", "", "", ErrValidation},
		{"blank hash", "alice@example.com", "  ", "", ErrValidation},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := NewUser(tc.email, tc.hash, t0)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Equal(t, User{}, u)
				return
			}
			require.NoError(t, err)
			require.NotEqual(t, uuid.Nil, u.ID)
			require.Equal(t, uuid.Version(7), u.ID.Version())
			require.Equal(t, tc.wantEmail, u.Email)
			require.Equal(t, tc.hash, u.PasswordHash)
			require.Equal(t, t0, u.CreatedAt)
			require.Equal(t, t0, u.UpdatedAt)
		})
	}
}

func TestParseEmail(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{"valid", "alice@example.com", "alice@example.com", nil},
		{"trims and lowercases", "  Alice@Example.COM \n", "alice@example.com", nil},
		{"empty", "", "", ErrValidation},
		{"blank", " \t ", "", ErrValidation},
		{"no at sign", "alice.example.com", "", ErrValidation},
		{"display name form", "Alice <alice@example.com>", "", ErrValidation},
		{"two addresses", "a@example.com, b@example.com", "", ErrValidation},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseEmail(tc.in)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Empty(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestNewUser_UniqueIDs(t *testing.T) {
	a, err := NewUser("a@example.com", "h", t0)
	require.NoError(t, err)
	b, err := NewUser("a@example.com", "h", t0)
	require.NoError(t, err)
	require.NotEqual(t, a.ID, b.ID)
}

func TestUser_ChangeEmail(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		wantEmail string
		wantErr   error
	}{
		{"valid", "bob@example.com", "bob@example.com", nil},
		{"normalizes", " BOB@Example.com ", "bob@example.com", nil},
		{"same email", "alice@example.com", "alice@example.com", nil},
		{"empty", "", "", ErrValidation},
		{"invalid", "not-an-email", "", ErrValidation},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := NewUser("alice@example.com", "h", t0)
			require.NoError(t, err)
			before := u

			err = u.ChangeEmail(tc.email, t1)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Equal(t, before, u, "user must be unchanged on error")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantEmail, u.Email)
			require.Equal(t, t1, u.UpdatedAt)
			require.Equal(t, t0, u.CreatedAt)
		})
	}
}

func TestUser_ChangePassword(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		wantErr error
	}{
		{"valid", "new-hash", nil},
		{"empty", "", ErrValidation},
		{"blank", "   ", ErrValidation},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := NewUser("alice@example.com", "old-hash", t0)
			require.NoError(t, err)
			before := u

			err = u.ChangePassword(tc.hash, t1)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Equal(t, before, u, "user must be unchanged on error")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.hash, u.PasswordHash)
			require.Equal(t, t1, u.UpdatedAt)
			require.Equal(t, t0, u.CreatedAt)
		})
	}
}

func TestUserErrors_WrapKinds(t *testing.T) {
	require.ErrorIs(t, ErrUserNotFound, ErrNotFound)
	require.ErrorIs(t, ErrEmailTaken, ErrConflict)
	// The message is sent to API clients as-is (architecture.md → Errors).
	require.Equal(t, "user not found", ErrUserNotFound.Error())
	require.Equal(t, "conflict: email already taken", ErrEmailTaken.Error())
}
