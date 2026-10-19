package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"connection_manager/domain"
)

var (
	t0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t1 = t0.Add(time.Hour)

	errDB   = errors.New("db down")
	errHash = errors.New("hasher down")
)

// --- fakes ---

type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time { return c.now }

type fakeHasher struct {
	err   error
	calls int
}

func (h *fakeHasher) Hash(_ context.Context, plain string) (string, error) {
	h.calls++
	if h.err != nil {
		return "", h.err
	}
	return "hashed:" + plain, nil
}

// fakeUserRepo is an in-memory UserRepository. Set err to make every call fail.
type fakeUserRepo struct {
	users   map[uuid.UUID]domain.User
	err     error
	updated int
}

func newFakeUserRepo(users ...domain.User) *fakeUserRepo {
	r := &fakeUserRepo{users: map[uuid.UUID]domain.User{}}
	for _, u := range users {
		r.users[u.ID] = u
	}
	return r
}

func (r *fakeUserRepo) Create(_ context.Context, u domain.User) error {
	if r.err != nil {
		return r.err
	}
	r.users[u.ID] = u
	return nil
}

func (r *fakeUserRepo) GetByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	if r.err != nil {
		return domain.User{}, r.err
	}
	u, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return u, nil
}

func (r *fakeUserRepo) List(_ context.Context) ([]domain.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	out := make([]domain.User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, u)
	}
	return out, nil
}

func (r *fakeUserRepo) Update(_ context.Context, u domain.User) error {
	if r.err != nil {
		return r.err
	}
	if _, ok := r.users[u.ID]; !ok {
		return domain.ErrUserNotFound
	}
	r.updated++
	r.users[u.ID] = u
	return nil
}

func (r *fakeUserRepo) Delete(_ context.Context, id uuid.UUID) error {
	if r.err != nil {
		return r.err
	}
	if _, ok := r.users[id]; !ok {
		return domain.ErrUserNotFound
	}
	delete(r.users, id)
	return nil
}

func existingUser(t *testing.T) domain.User {
	t.Helper()
	u, err := domain.NewUser("alice@example.com", "hashed:old-password", t0)
	require.NoError(t, err)
	return u
}

func ptr(s string) *string { return &s }

// --- tests ---

func TestUserUsecase_Create(t *testing.T) {
	tests := []struct {
		name    string
		in      CreateUserInput
		repoErr error
		hashErr error
		wantErr error
	}{
		{name: "happy path", in: CreateUserInput{Email: " Alice@Example.com ", Password: "secret123"}},
		{name: "domain error", in: CreateUserInput{Email: "nope", Password: "secret123"}, wantErr: domain.ErrValidation},
		{name: "email taken", in: CreateUserInput{Email: "a@example.com", Password: "secret123"}, repoErr: domain.ErrEmailTaken, wantErr: domain.ErrEmailTaken},
		{name: "repository error", in: CreateUserInput{Email: "a@example.com", Password: "secret123"}, repoErr: errDB, wantErr: errDB},
		{name: "hasher error", in: CreateUserInput{Email: "a@example.com", Password: "secret123"}, hashErr: errHash, wantErr: errHash},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeUserRepo()
			repo.err = tc.repoErr
			uc := NewUserUsecase(repo, &fakeHasher{err: tc.hashErr}, &fixedClock{now: t0})

			got, err := uc.Create(context.Background(), tc.in)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Equal(t, domain.User{}, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "alice@example.com", got.Email)
			require.Equal(t, "hashed:secret123", got.PasswordHash)
			require.Equal(t, t0, got.CreatedAt)
			require.Equal(t, got, repo.users[got.ID])
		})
	}
}

func TestUserUsecase_Get(t *testing.T) {
	u := existingUser(t)
	tests := []struct {
		name    string
		id      uuid.UUID
		repoErr error
		wantErr error
	}{
		{name: "found", id: u.ID},
		{name: "not found", id: uuid.Must(uuid.NewV7()), wantErr: domain.ErrUserNotFound},
		{name: "repository error", id: u.ID, repoErr: errDB, wantErr: errDB},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeUserRepo(u)
			repo.err = tc.repoErr
			uc := NewUserUsecase(repo, &fakeHasher{}, &fixedClock{now: t1})

			got, err := uc.Get(context.Background(), tc.id)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, u, got)
		})
	}
}

func TestUserUsecase_List(t *testing.T) {
	u := existingUser(t)

	t.Run("returns users", func(t *testing.T) {
		uc := NewUserUsecase(newFakeUserRepo(u), &fakeHasher{}, &fixedClock{now: t1})
		got, err := uc.List(context.Background())
		require.NoError(t, err)
		require.Equal(t, []domain.User{u}, got)
	})

	t.Run("repository error", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.err = errDB
		_, err := NewUserUsecase(repo, &fakeHasher{}, &fixedClock{now: t1}).List(context.Background())
		require.ErrorIs(t, err, errDB)
	})
}

func TestUserUsecase_Update(t *testing.T) {
	tests := []struct {
		name       string
		in         func(id uuid.UUID) UpdateUserInput
		repoErr    error
		hashErr    error
		wantErr    error
		wantEmail  string
		wantHash   string
		wantHashed int
	}{
		{
			name:      "email only",
			in:        func(id uuid.UUID) UpdateUserInput { return UpdateUserInput{ID: id, Email: ptr("BOB@example.com")} },
			wantEmail: "bob@example.com", wantHash: "hashed:old-password",
		},
		{
			name:      "password only",
			in:        func(id uuid.UUID) UpdateUserInput { return UpdateUserInput{ID: id, Password: ptr("new-password")} },
			wantEmail: "alice@example.com", wantHash: "hashed:new-password", wantHashed: 1,
		},
		{
			name: "both",
			in: func(id uuid.UUID) UpdateUserInput {
				return UpdateUserInput{ID: id, Email: ptr("bob@example.com"), Password: ptr("new-password")}
			},
			wantEmail: "bob@example.com", wantHash: "hashed:new-password", wantHashed: 1,
		},
		{
			name: "not found",
			in: func(uuid.UUID) UpdateUserInput {
				return UpdateUserInput{ID: uuid.Must(uuid.NewV7()), Email: ptr("b@example.com")}
			},
			wantErr: domain.ErrUserNotFound,
		},
		{
			name:    "domain error",
			in:      func(id uuid.UUID) UpdateUserInput { return UpdateUserInput{ID: id, Email: ptr("not-an-email")} },
			wantErr: domain.ErrValidation,
		},
		{
			name:    "repository error",
			in:      func(id uuid.UUID) UpdateUserInput { return UpdateUserInput{ID: id, Email: ptr("b@example.com")} },
			repoErr: errDB, wantErr: errDB,
		},
		{
			name:    "hasher error",
			in:      func(id uuid.UUID) UpdateUserInput { return UpdateUserInput{ID: id, Password: ptr("new-password")} },
			hashErr: errHash, wantErr: errHash, wantHashed: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			orig := existingUser(t)
			repo := newFakeUserRepo(orig)
			repo.err = tc.repoErr
			hasher := &fakeHasher{err: tc.hashErr}
			uc := NewUserUsecase(repo, hasher, &fixedClock{now: t1})

			got, err := uc.Update(context.Background(), tc.in(orig.ID))
			require.Equal(t, tc.wantHashed, hasher.calls)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Equal(t, 0, repo.updated, "nothing must be persisted on error")
				return
			}
			require.NoError(t, err)
			require.Equal(t, orig.ID, got.ID)
			require.Equal(t, tc.wantEmail, got.Email)
			require.Equal(t, tc.wantHash, got.PasswordHash)
			require.Equal(t, t0, got.CreatedAt)
			require.Equal(t, t1, got.UpdatedAt)
			require.Equal(t, got, repo.users[orig.ID])
		})
	}
}

func TestUserUsecase_Update_EmailTaken(t *testing.T) {
	orig := existingUser(t)
	repo := &takenOnUpdateRepo{fakeUserRepo: newFakeUserRepo(orig)}
	uc := NewUserUsecase(repo, &fakeHasher{}, &fixedClock{now: t1})

	_, err := uc.Update(context.Background(), UpdateUserInput{ID: orig.ID, Email: ptr("taken@example.com")})
	require.ErrorIs(t, err, domain.ErrEmailTaken)
}

// takenOnUpdateRepo loads normally but rejects Update with ErrEmailTaken,
// as the real repository does on a unique violation.
type takenOnUpdateRepo struct{ *fakeUserRepo }

func (r *takenOnUpdateRepo) Update(context.Context, domain.User) error { return domain.ErrEmailTaken }

func TestUserUsecase_Delete(t *testing.T) {
	u := existingUser(t)
	tests := []struct {
		name    string
		id      uuid.UUID
		repoErr error
		wantErr error
	}{
		{name: "deletes", id: u.ID},
		{name: "not found", id: uuid.Must(uuid.NewV7()), wantErr: domain.ErrUserNotFound},
		{name: "repository error", id: u.ID, repoErr: errDB, wantErr: errDB},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeUserRepo(u)
			repo.err = tc.repoErr
			uc := NewUserUsecase(repo, &fakeHasher{}, &fixedClock{now: t1})

			err := uc.Delete(context.Background(), tc.id)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotContains(t, repo.users, u.ID)
		})
	}
}
