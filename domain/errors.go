package domain

import (
	"errors"
	"fmt"
)

// Kind sentinels. Every domain error wraps exactly one of these.
var (
	ErrValidation   = errors.New("validation failed")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalidState = errors.New("invalid state")
	ErrForbidden    = errors.New("forbidden")
)

// Entity-specific errors.
var (
	ErrUserNotFound = fmt.Errorf("user %w", ErrNotFound)
	ErrEmailTaken   = fmt.Errorf("email already taken: %w", ErrConflict)
)
