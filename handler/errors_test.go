package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"connection_manager/domain"
	"connection_manager/dto"
)

// newTestApp builds a Fiber app configured like router.New (error handler + validator).
func newTestApp() *fiber.App {
	return fiber.New(fiber.Config{
		ErrorHandler:    ErrorHandler,
		StructValidator: NewStructValidator(),
	})
}

func decodeError(t *testing.T, body io.Reader) dto.ErrorResponse {
	t.Helper()
	var out dto.ErrorResponse
	require.NoError(t, json.NewDecoder(body).Decode(&out))
	return out
}

type validated struct {
	Name string `json:"name" validate:"required"`
}

func TestErrorHandler(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{"fiber error", fiber.NewError(http.StatusBadRequest, "invalid id"), 400, "http_error", "invalid id"},
		{"domain validation", fmt.Errorf("%w: email is invalid", domain.ErrValidation), 400, "validation_error", "validation failed: email is invalid"},
		{"not found", domain.ErrUserNotFound, 404, "not_found", "user not found"},
		{"wrapped not found", fmt.Errorf("get: %w", domain.ErrNotFound), 404, "not_found", "get: not found"},
		{"conflict", domain.ErrEmailTaken, 409, "conflict", "email already taken: conflict"},
		{"invalid state", domain.ErrInvalidState, 409, "invalid_state", "invalid state"},
		{"forbidden", domain.ErrForbidden, 403, "forbidden", "forbidden"},
		{"unknown", errors.New("db exploded"), 500, "internal_error", "internal server error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestApp()
			app.Get("/", func(fiber.Ctx) error { return tc.err })

			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, resp.StatusCode)
			body := decodeError(t, resp.Body)
			require.Equal(t, tc.wantCode, body.Error.Code)
			require.Equal(t, tc.wantMsg, body.Error.Message)
		})
	}
}

func TestErrorHandler_BindAndValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"malformed json", `{"name":`},
		{"validation failure", `{}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestApp()
			app.Post("/", func(c fiber.Ctx) error {
				var v validated
				return c.Bind().Body(&v)
			})
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
			require.Equal(t, "invalid_request", decodeError(t, resp.Body).Error.Code)
		})
	}
}
