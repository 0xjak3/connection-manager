package service

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"connection_manager/domain"
)

type PasswordConfig struct {
	// Cost is the bcrypt cost. Zero means bcrypt.DefaultCost.
	Cost int
}

// PasswordService implements usecase.PasswordHasher with bcrypt.
type PasswordService struct{ cost int }

func NewPasswordService(cfg PasswordConfig) *PasswordService {
	cost := cfg.Cost
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return &PasswordService{cost: cost}
}

func (s *PasswordService) Hash(ctx context.Context, plain string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("service: hash password: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), s.cost)
	if err != nil {
		// bcrypt limits input to 72 bytes; multi-byte characters can exceed it
		// even when the dto's rune-based max=72 passes.
		if errors.Is(err, bcrypt.ErrPasswordTooLong) {
			return "", fmt.Errorf("%w: password must be at most 72 bytes", domain.ErrValidation)
		}
		return "", fmt.Errorf("service: hash password: %w", err)
	}
	return string(hash), nil
}
