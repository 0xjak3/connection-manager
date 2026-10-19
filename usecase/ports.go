package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"connection_manager/domain"
)

type Clock interface{ Now() time.Time }

type UserRepository interface {
	Create(ctx context.Context, u domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	List(ctx context.Context) ([]domain.User, error)
	Update(ctx context.Context, u domain.User) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// PasswordHasher turns a plain password into a one-way hash. Plain passwords
// never go past the usecase layer.
type PasswordHasher interface {
	Hash(ctx context.Context, plain string) (string, error)
}
