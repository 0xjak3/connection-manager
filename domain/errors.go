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

// user
var (
	ErrUserNotFound = fmt.Errorf("user %w", ErrNotFound)
	ErrEmailTaken   = fmt.Errorf("%w: email already taken", ErrConflict)
	ErrNoChanges    = fmt.Errorf("%w: at least one of email or password is required", ErrValidation)
)
