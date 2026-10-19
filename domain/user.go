package domain

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User is an account identified by a unique email. The password is only ever
// held as a hash; hashing is done by a service before the domain sees it.
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewUser(email, passwordHash string, now time.Time) (User, error) {
	email, err := ParseEmail(email)
	if err != nil {
		return User{}, err
	}
	if err := validatePasswordHash(passwordHash); err != nil {
		return User{}, err
	}
	return User{
		ID:           uuid.Must(uuid.NewV7()),
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// UserChanges is one update to a user. Nil fields are left as they are.
type UserChanges struct {
	Email        *string
	PasswordHash *string
}

// IsEmpty reports whether the changes would modify nothing.
func (c UserChanges) IsEmpty() bool { return c.Email == nil && c.PasswordHash == nil }

// Update validates every change first, then applies them all and records a
// single UpdatedAt. On error the user is left unchanged.
func (u *User) Update(c UserChanges, now time.Time) error {
	if c.IsEmpty() {
		return ErrNoChanges
	}
	email := u.Email
	if c.Email != nil {
		var err error
		if email, err = ParseEmail(*c.Email); err != nil {
			return err
		}
	}
	hash := u.PasswordHash
	if c.PasswordHash != nil {
		if err := validatePasswordHash(*c.PasswordHash); err != nil {
			return err
		}
		hash = *c.PasswordHash
	}
	u.Email = email
	u.PasswordHash = hash
	u.touch(now)
	return nil
}

func (u *User) ChangeEmail(email string, now time.Time) error {
	return u.Update(UserChanges{Email: &email}, now)
}

func (u *User) ChangePassword(passwordHash string, now time.Time) error {
	return u.Update(UserChanges{PasswordHash: &passwordHash}, now)
}

// touch records a successful change. UpdatedAt always moves strictly forward,
// even if now is not after it (same microsecond, or the clock stepped back).
func (u *User) touch(now time.Time) {
	now = now.Truncate(time.Microsecond) // Postgres timestamptz precision
	if !now.After(u.UpdatedAt) {
		now = u.UpdatedAt.Add(time.Microsecond)
	}
	u.UpdatedAt = now
}

// ParseEmail trims and lowercases the email and checks it is a bare
// addr-spec ("local@domain"), rejecting display names and address lists.
// It is pure, so a usecase can reject bad input before expensive work.
func ParseEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", fmt.Errorf("%w: email is required", ErrValidation)
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Name != "" || addr.Address != email {
		return "", fmt.Errorf("%w: email is invalid", ErrValidation)
	}
	return email, nil
}

func validatePasswordHash(hash string) error {
	if strings.TrimSpace(hash) == "" {
		return fmt.Errorf("%w: password hash is required", ErrValidation)
	}
	return nil
}
