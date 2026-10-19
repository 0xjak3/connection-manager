package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"connection_manager/domain"
)

func TestPasswordService_Hash(t *testing.T) {
	svc := NewPasswordService(PasswordConfig{Cost: bcrypt.MinCost})

	tests := []struct {
		name    string
		plain   string
		wantErr error
	}{
		{"ascii", "secret123", nil},
		{"72 bytes", strings.Repeat("a", 72), nil},
		{"over 72 bytes", strings.Repeat("é", 37), domain.ErrValidation}, // 37 runes, 74 bytes
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := svc.Hash(context.Background(), tc.plain)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotEqual(t, tc.plain, hash)
			require.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte(tc.plain)))
		})
	}
}

func TestPasswordService_Hash_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewPasswordService(PasswordConfig{Cost: bcrypt.MinCost}).Hash(ctx, "secret123")
	require.ErrorIs(t, err, context.Canceled)
}
