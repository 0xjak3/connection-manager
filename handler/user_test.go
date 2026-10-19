package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"connection_manager/domain"
	"connection_manager/usecase"
)

type fakeUserUsecase struct {
	create func(ctx context.Context, in usecase.CreateUserInput) (domain.User, error)
	get    func(ctx context.Context, id uuid.UUID) (domain.User, error)
	list   func(ctx context.Context) ([]domain.User, error)
	update func(ctx context.Context, in usecase.UpdateUserInput) (domain.User, error)
	delete func(ctx context.Context, id uuid.UUID) error
}

func (f *fakeUserUsecase) Create(ctx context.Context, in usecase.CreateUserInput) (domain.User, error) {
	return f.create(ctx, in)
}
func (f *fakeUserUsecase) Get(ctx context.Context, id uuid.UUID) (domain.User, error) {
	return f.get(ctx, id)
}
func (f *fakeUserUsecase) List(ctx context.Context) ([]domain.User, error) { return f.list(ctx) }
func (f *fakeUserUsecase) Update(ctx context.Context, in usecase.UpdateUserInput) (domain.User, error) {
	return f.update(ctx, in)
}
func (f *fakeUserUsecase) Delete(ctx context.Context, id uuid.UUID) error { return f.delete(ctx, id) }

var (
	fixedID   = uuid.MustParse("01920000-0000-7000-8000-000000000001")
	fixedTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	testUser  = domain.User{
		ID:           fixedID,
		Email:        "alice@example.com",
		PasswordHash: "SECRET-HASH",
		CreatedAt:    fixedTime,
		UpdatedAt:    fixedTime,
	}
	testUserJSON = `{"id":"01920000-0000-7000-8000-000000000001","email":"alice@example.com",` +
		`"created_at":"2026-01-02T03:04:05Z","updated_at":"2026-01-02T03:04:05Z"}`
)

func newUserTestApp(uc userUsecase) *fiber.App {
	app := newTestApp()
	h := NewUserHandler(uc)
	app.Post("/users", h.Create)
	app.Get("/users", h.List)
	app.Get("/users/:id", h.Get)
	app.Patch("/users/:id", h.Update)
	app.Delete("/users/:id", h.Delete)
	return app
}

func doRequest(t *testing.T, app *fiber.App, method, path, body string) (int, string) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// Responses must never expose passwords or hashes.
	require.NotContains(t, string(b), "SECRET-HASH")
	require.NotContains(t, string(b), "secret123")
	require.NotContains(t, string(b), "new-secret")
	if resp.StatusCode < 300 {
		require.NotContains(t, strings.ToLower(string(b)), "password")
	}
	return resp.StatusCode, string(b)
}

func errorCode(t *testing.T, body string) string {
	t.Helper()
	return decodeError(t, strings.NewReader(body)).Error.Code
}

func TestUserHandler_Create(t *testing.T) {
	var got usecase.CreateUserInput
	ok := &fakeUserUsecase{create: func(_ context.Context, in usecase.CreateUserInput) (domain.User, error) {
		got = in
		return testUser, nil
	}}
	taken := &fakeUserUsecase{create: func(context.Context, usecase.CreateUserInput) (domain.User, error) {
		return domain.User{}, domain.ErrEmailTaken
	}}

	tests := []struct {
		name       string
		uc         userUsecase
		body       string
		wantStatus int
		wantCode   string
		wantEmail  string // passed to the usecase as sent; the domain normalizes it
	}{
		{"success", ok, `{"email":"alice@example.com","password":"secret123"}`, 201, "", "alice@example.com"},
		{"padded email", ok, `{"email":"  Alice@Example.com ","password":"secret123"}`, 201, "", "  Alice@Example.com "},
		{"missing email", ok, `{"password":"secret123"}`, 400, "invalid_request", ""},
		{"blank email", ok, `{"email":"   ","password":"secret123"}`, 400, "invalid_request", ""},
		{"bad email", ok, `{"email":"nope","password":"secret123"}`, 400, "invalid_request", ""},
		{"padded bad email", ok, `{"email":" nope ","password":"secret123"}`, 400, "invalid_request", ""},
		{"short password", ok, `{"email":"alice@example.com","password":"short"}`, 400, "invalid_request", ""},
		{"long password", ok, `{"email":"alice@example.com","password":"` + strings.Repeat("a", 73) + `"}`, 400, "invalid_request", ""},
		{"malformed json", ok, `{"email":`, 400, "invalid_request", ""},
		{"email taken", taken, `{"email":"alice@example.com","password":"secret123"}`, 409, "conflict", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got = usecase.CreateUserInput{}
			status, body := doRequest(t, newUserTestApp(tc.uc), http.MethodPost, "/users", tc.body)
			require.Equal(t, tc.wantStatus, status, body)
			if tc.wantCode != "" {
				require.Equal(t, tc.wantCode, errorCode(t, body))
				return
			}
			require.JSONEq(t, `{"data":`+testUserJSON+`}`, body)
			require.Equal(t, usecase.CreateUserInput{Email: tc.wantEmail, Password: "secret123"}, got)
		})
	}
}

func TestUserHandler_Create_WithoutContentType(t *testing.T) {
	uc := &fakeUserUsecase{create: func(context.Context, usecase.CreateUserInput) (domain.User, error) {
		t.Fatal("usecase must not be called")
		return domain.User{}, nil
	}}
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"email":"alice@example.com","password":"secret123"}`))
	resp, err := newUserTestApp(uc).Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, "invalid_request", decodeError(t, resp.Body).Error.Code)
}

func TestUserHandler_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		uc := &fakeUserUsecase{list: func(context.Context) ([]domain.User, error) { return []domain.User{testUser}, nil }}
		status, body := doRequest(t, newUserTestApp(uc), http.MethodGet, "/users", "")
		require.Equal(t, 200, status)
		require.JSONEq(t, `{"data":[`+testUserJSON+`]}`, body)
	})

	t.Run("empty is an empty array", func(t *testing.T) {
		uc := &fakeUserUsecase{list: func(context.Context) ([]domain.User, error) { return nil, nil }}
		status, body := doRequest(t, newUserTestApp(uc), http.MethodGet, "/users", "")
		require.Equal(t, 200, status)
		require.JSONEq(t, `{"data":[]}`, body)
	})
}

func TestUserHandler_Get(t *testing.T) {
	uc := &fakeUserUsecase{get: func(_ context.Context, id uuid.UUID) (domain.User, error) {
		if id == fixedID {
			return testUser, nil
		}
		return domain.User{}, domain.ErrUserNotFound
	}}
	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantCode   string
	}{
		{"found", "/users/" + fixedID.String(), 200, ""},
		{"not found", "/users/" + uuid.Must(uuid.NewV7()).String(), 404, "not_found"},
		{"invalid id", "/users/not-a-uuid", 400, "http_error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, body := doRequest(t, newUserTestApp(uc), http.MethodGet, tc.path, "")
			require.Equal(t, tc.wantStatus, status, body)
			if tc.wantCode != "" {
				require.Equal(t, tc.wantCode, errorCode(t, body))
				return
			}
			require.JSONEq(t, `{"data":`+testUserJSON+`}`, body)
		})
	}
}

func TestUserHandler_Update(t *testing.T) {
	var got usecase.UpdateUserInput
	uc := &fakeUserUsecase{update: func(_ context.Context, in usecase.UpdateUserInput) (domain.User, error) {
		got = in
		if in.ID != fixedID {
			return domain.User{}, domain.ErrUserNotFound
		}
		return testUser, nil
	}}
	path := "/users/" + fixedID.String()

	tests := []struct {
		name         string
		path         string
		body         string
		wantStatus   int
		wantCode     string
		wantEmail    *string
		wantPassword *string
	}{
		{name: "email only", path: path, body: `{"email":"bob@example.com"}`, wantStatus: 200, wantEmail: ptr("bob@example.com")},
		{name: "padded email", path: path, body: `{"email":" BOB@example.com  "}`, wantStatus: 200, wantEmail: ptr(" BOB@example.com  ")},
		{name: "blank email", path: path, body: `{"email":"  "}`, wantStatus: 400, wantCode: "invalid_request"},
		{name: "password only", path: path, body: `{"password":"new-secret"}`, wantStatus: 200, wantPassword: ptr("new-secret")},
		{name: "both", path: path, body: `{"email":"bob@example.com","password":"new-secret"}`, wantStatus: 200, wantEmail: ptr("bob@example.com"), wantPassword: ptr("new-secret")},
		{name: "neither", path: path, body: `{}`, wantStatus: 400, wantCode: "invalid_request"},
		{name: "explicit nulls", path: path, body: `{"email":null,"password":null}`, wantStatus: 400, wantCode: "invalid_request"},
		{name: "bad email", path: path, body: `{"email":"nope"}`, wantStatus: 400, wantCode: "invalid_request"},
		{name: "empty email", path: path, body: `{"email":""}`, wantStatus: 400, wantCode: "invalid_request"},
		{name: "short password", path: path, body: `{"password":"short"}`, wantStatus: 400, wantCode: "invalid_request"},
		{name: "invalid id", path: "/users/xyz", body: `{"email":"bob@example.com"}`, wantStatus: 400, wantCode: "http_error"},
		{name: "not found", path: "/users/" + uuid.Must(uuid.NewV7()).String(), body: `{"email":"bob@example.com"}`, wantStatus: 404, wantCode: "not_found"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got = usecase.UpdateUserInput{}
			status, body := doRequest(t, newUserTestApp(uc), http.MethodPatch, tc.path, tc.body)
			require.Equal(t, tc.wantStatus, status, body)
			if tc.wantCode != "" {
				require.Equal(t, tc.wantCode, errorCode(t, body))
				return
			}
			require.JSONEq(t, `{"data":`+testUserJSON+`}`, body)
			require.Equal(t, usecase.UpdateUserInput{ID: fixedID, Email: tc.wantEmail, Password: tc.wantPassword}, got)
		})
	}
}

func TestUserHandler_Update_EmptyMessageUsesJSONNames(t *testing.T) {
	uc := &fakeUserUsecase{update: func(context.Context, usecase.UpdateUserInput) (domain.User, error) {
		t.Fatal("usecase must not be called")
		return domain.User{}, nil
	}}
	status, body := doRequest(t, newUserTestApp(uc), http.MethodPatch, "/users/"+fixedID.String(), `{}`)
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "email failed required_without=password; password failed required_without=email",
		decodeError(t, strings.NewReader(body)).Error.Message)
}

func TestUserHandler_Delete(t *testing.T) {
	uc := &fakeUserUsecase{delete: func(_ context.Context, id uuid.UUID) error {
		if id == fixedID {
			return nil
		}
		return domain.ErrUserNotFound
	}}
	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantCode   string
	}{
		{"deleted", "/users/" + fixedID.String(), 204, ""},
		{"not found", "/users/" + uuid.Must(uuid.NewV7()).String(), 404, "not_found"},
		{"invalid id", "/users/xyz", 400, "http_error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, body := doRequest(t, newUserTestApp(uc), http.MethodDelete, tc.path, "")
			require.Equal(t, tc.wantStatus, status, body)
			if tc.wantCode != "" {
				require.Equal(t, tc.wantCode, errorCode(t, body))
				return
			}
			require.Empty(t, body)
		})
	}
}

func ptr(s string) *string { return &s }

// Guard against accidentally adding a password field to the response dto.
func TestUserResponse_HasNoPasswordField(t *testing.T) {
	b, err := json.Marshal(toUserResponse(testUser))
	require.NoError(t, err)
	require.JSONEq(t, testUserJSON, string(b))
}
