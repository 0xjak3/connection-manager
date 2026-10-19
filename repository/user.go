package repository

import (
	"context"

	"github.com/google/uuid"

	"connection_manager/domain"
	"connection_manager/repository/sqlcgen"
)

type UserRepository struct{ q *sqlcgen.Queries }

func NewUserRepository(db sqlcgen.DBTX) *UserRepository {
	return &UserRepository{q: sqlcgen.New(db)}
}

func (r *UserRepository) Create(ctx context.Context, u domain.User) error {
	if err := r.q.CreateUser(ctx, createUserParams(u)); err != nil {
		return translateErr("create user", err, nil, domain.ErrEmailTaken)
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return domain.User{}, translateErr("get user", err, domain.ErrUserNotFound, nil)
	}
	return userFromRow(row), nil
}

func (r *UserRepository) List(ctx context.Context) ([]domain.User, error) {
	rows, err := r.q.ListUsers(ctx)
	if err != nil {
		return nil, translateErr("list users", err, nil, nil)
	}
	users := make([]domain.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, userFromRow(row))
	}
	return users, nil
}

func (r *UserRepository) Update(ctx context.Context, u domain.User) error {
	n, err := r.q.UpdateUser(ctx, updateUserParams(u))
	if err != nil {
		return translateErr("update user", err, domain.ErrUserNotFound, domain.ErrEmailTaken)
	}
	if n == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := r.q.DeleteUser(ctx, id)
	if err != nil {
		return translateErr("delete user", err, domain.ErrUserNotFound, nil)
	}
	if n == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// userFromRow rebuilds a trusted entity from a row. pgx returns timestamptz in
// the process's local zone; normalize to UTC to match service.SystemClock.
func userFromRow(row sqlcgen.User) domain.User {
	return domain.User{
		ID:           row.ID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		CreatedAt:    row.CreatedAt.UTC(),
		UpdatedAt:    row.UpdatedAt.UTC(),
	}
}

func createUserParams(u domain.User) sqlcgen.CreateUserParams {
	return sqlcgen.CreateUserParams{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

func updateUserParams(u domain.User) sqlcgen.UpdateUserParams {
	return sqlcgen.UpdateUserParams{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		UpdatedAt:    u.UpdatedAt,
	}
}
