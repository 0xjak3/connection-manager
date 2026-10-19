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
	hash, err := u.hasher.Hash(ctx, in.Password)
	if err != nil {
		return domain.User{}, err
	}
	user, err := domain.NewUser(in.Email, hash, u.clock.Now())
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
	user, err := u.repo.GetByID(ctx, in.ID)
	if err != nil {
		return domain.User{}, err
	}
	now := u.clock.Now()
	if in.Email != nil {
		if err := user.ChangeEmail(*in.Email, now); err != nil {
			return domain.User{}, err
		}
	}
	if in.Password != nil {
		hash, err := u.hasher.Hash(ctx, *in.Password)
		if err != nil {
			return domain.User{}, err
		}
		if err := user.ChangePassword(hash, now); err != nil {
			return domain.User{}, err
		}
	}
	if err := u.repo.Update(ctx, user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (u *UserUsecase) Delete(ctx context.Context, id uuid.UUID) error {
	return u.repo.Delete(ctx, id)
}
