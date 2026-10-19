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

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"connection_manager/domain"
	"connection_manager/dto"
)

// newTestApp builds a Fiber app with the same config as router.New.
func newTestApp() *fiber.App {
	return fiber.New(FiberConfig())
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
		{"conflict", domain.ErrEmailTaken, 409, "conflict", "conflict: email already taken"},
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
		name        string
		body        string
		contentType string
		wantMsg     string
	}{
		{"validation failure", `{}`, "application/json", "name failed required"},
		{"malformed json", `{"name":`, "application/json", "malformed request body"},
		{"missing content type", `{"name":"x"}`, "", "unsupported or missing Content-Type"},
		{"unsupported content type", `name=x`, "application/x-unknown", "unsupported or missing Content-Type"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestApp()
			app.Post("/", func(c fiber.Ctx) error {
				var v validated
				return c.Bind().Body(&v)
			})
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
			body := decodeError(t, resp.Body)
			require.Equal(t, "invalid_request", body.Error.Code)
			require.Equal(t, tc.wantMsg, body.Error.Message)
		})
	}
}

// crossField has rules whose params reference other fields by Go name.
type crossField struct {
	FirstName *string `json:"first_name" validate:"required_without=UserID"`
	UserID    *string `json:"user_id" validate:"required_without=FirstName"`
	Kind      string  `json:"kind"`
	HTTPAddr  string  `json:"http_addr" validate:"required_if=Kind server"`
	Confirm   string  `json:"confirm" validate:"eqfield=HTTPAddr"`
}

func TestValidationMessage_UsesJSONNamesInParams(t *testing.T) {
	err := NewStructValidator().Validate(crossField{Kind: "server", Confirm: "x"})
	var verrs validator.ValidationErrors
	require.ErrorAs(t, err, &verrs)

	msg := validationMessage(verrs)
	require.Equal(t, "first_name failed required_without=user_id; "+
		"user_id failed required_without=first_name; "+
		"http_addr failed required_if=kind server; "+
		"confirm failed eqfield=http_addr", msg)
}

func TestSnakeCase(t *testing.T) {
	tests := map[string]string{
		"Password": "password", "Email": "email", "CreatedAt": "created_at",
		"UserID": "user_id", "HTTPAddr": "http_addr", "ID": "id", "V2Name": "v2_name",
	}
	for in, want := range tests {
		t.Run(in, func(t *testing.T) { require.Equal(t, want, snakeCase(in)) })
	}
}
