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
	email, err := normalizeEmail(email)
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

func (u *User) ChangeEmail(email string, now time.Time) error {
	email, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	u.Email = email
	u.UpdatedAt = now
	return nil
}

func (u *User) ChangePassword(passwordHash string, now time.Time) error {
	if err := validatePasswordHash(passwordHash); err != nil {
		return err
	}
	u.PasswordHash = passwordHash
	u.UpdatedAt = now
	return nil
}

// normalizeEmail trims and lowercases the email and checks it is a bare
// addr-spec ("local@domain"), rejecting display names and address lists.
func normalizeEmail(email string) (string, error) {
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
