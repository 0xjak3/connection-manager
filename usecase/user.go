package usecase

import (
	"context"

	"github.com/google/uuid"

	"connection_manager/domain"
)

type UserUsecase struct {
	repo   UserRepository
	hasher PasswordHasher
	clock  Clock
}

func NewUserUsecase(repo UserRepository, hasher PasswordHasher, clock Clock) *UserUsecase {
	return &UserUsecase{repo: repo, hasher: hasher, clock: clock}
}

type CreateUserInput struct {
	Email    string
	Password string
}

// UpdateUserInput changes only the fields that are non-nil.
type UpdateUserInput struct {
	ID       uuid.UUID
	Email    *string
	Password *string
}

func (u *UserUsecase) Create(ctx context.Context, in CreateUserInput) (domain.User, error) {
	email, err := domain.ParseEmail(in.Email) // cheap check before hashing
	if err != nil {
		return domain.User{}, err
	}
	hash, err := u.hasher.Hash(ctx, in.Password)
	if err != nil {
		return domain.User{}, err
	}
	user, err := domain.NewUser(email, hash, u.clock.Now())
	if err != nil {
		return domain.User{}, err
	}
	if err := u.repo.Create(ctx, user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (u *UserUsecase) Get(ctx context.Context, id uuid.UUID) (domain.User, error) {
	return u.repo.GetByID(ctx, id)
}

func (u *UserUsecase) List(ctx context.Context) ([]domain.User, error) {
	return u.repo.List(ctx)
}

func (u *UserUsecase) Update(ctx context.Context, in UpdateUserInput) (domain.User, error) {
	// Cheap checks before loading and hashing; the domain re-checks on Update.
	if (domain.UserChanges{Email: in.Email, PasswordHash: in.Password}).IsEmpty() {
		return domain.User{}, domain.ErrNoChanges
	}
	if in.Email != nil {
		if _, err := domain.ParseEmail(*in.Email); err != nil {
			return domain.User{}, err
		}
	}
	user, err := u.repo.GetByID(ctx, in.ID)
	if err != nil {
		return domain.User{}, err
	}
	changes := domain.UserChanges{Email: in.Email}
	if in.Password != nil {
		hash, err := u.hasher.Hash(ctx, *in.Password)
		if err != nil {
			return domain.User{}, err
		}
		changes.PasswordHash = &hash
	}
	if err := user.Update(changes, u.clock.Now()); err != nil {
		return domain.User{}, err
	}
	if err := u.repo.Update(ctx, user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (u *UserUsecase) Delete(ctx context.Context, id uuid.UUID) error {
	return u.repo.Delete(ctx, id)
}
