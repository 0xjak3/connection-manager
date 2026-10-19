//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const usersPath = "/api/v1/users"

func userPath(id string) string { return usersPath + "/" + id }

func TestHealthz(t *testing.T) {
	t.Parallel()
	status, raw := doRaw(t, http.MethodGet, "/healthz", nil, "")
	assert.Equal(t, http.StatusOK, status, "body: %s", raw)
}

func TestUser_SignUpAndManageAccount(t *testing.T) {
	t.Parallel()
	email := uniqueEmail(t)
	password := "s3cret-passw0rd"
	var created user

	t.Run("sign up", func(t *testing.T) {
		created = signUp(t, email, password)
		assert.Equal(t, email, created.Email)
		assert.True(t, created.CreatedAt.Equal(created.UpdatedAt), "created_at %v != updated_at %v", created.CreatedAt, created.UpdatedAt)
	})
	require.NotEmpty(t, created.ID, "sign up failed, cannot continue")

	t.Run("fetch it back", func(t *testing.T) {
		status, raw := doJSON(t, http.MethodGet, userPath(created.ID), nil)
		requireStatus(t, status, http.StatusOK, raw)
		assertNoSecrets(t, raw, password)
		got := parseUser(t, dataObject(t, decode(t, raw)))
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, email, got.Email)
		assert.True(t, created.CreatedAt.Equal(got.CreatedAt))
		assert.True(t, created.UpdatedAt.Equal(got.UpdatedAt))
	})

	t.Run("see it in the user list", func(t *testing.T) {
		status, raw := doJSON(t, http.MethodGet, usersPath, nil)
		requireStatus(t, status, http.StatusOK, raw)
		assertNoSecrets(t, raw, password)
		list, ok := decode(t, raw)["data"].([]any)
		require.True(t, ok, `"data" is not an array: %s`, raw)
		found := false
		for _, item := range list {
			obj, ok := item.(map[string]any)
			require.True(t, ok, "list item is not an object: %v", item)
			u := parseUser(t, obj)
			if u.ID == created.ID {
				found = true
				assert.Equal(t, email, u.Email)
			}
		}
		assert.True(t, found, "created user %s not in list", created.ID)
	})

	newEmail := uniqueEmail(t)
	var afterEmail user

	t.Run("change email", func(t *testing.T) {
		status, raw := doJSON(t, http.MethodPatch, userPath(created.ID), map[string]any{"email": strings.ToUpper(newEmail)})
		requireStatus(t, status, http.StatusOK, raw)
		assertNoSecrets(t, raw, password)
		afterEmail = parseUser(t, dataObject(t, decode(t, raw)))
		assert.Equal(t, newEmail, afterEmail.Email, "email must be normalized to lowercase")
		assert.Equal(t, created.ID, afterEmail.ID)
		assert.True(t, created.CreatedAt.Equal(afterEmail.CreatedAt), "created_at changed")
		assert.False(t, afterEmail.UpdatedAt.Before(created.UpdatedAt), "updated_at went back in time")
	})

	t.Run("fetch shows the new email", func(t *testing.T) {
		status, body := do(t, http.MethodGet, userPath(created.ID), nil)
		requireStatus(t, status, http.StatusOK, body)
		assert.Equal(t, newEmail, parseUser(t, dataObject(t, body)).Email)
	})

	t.Run("change password", func(t *testing.T) {
		newPassword := "another-s3cret"
		status, raw := doJSON(t, http.MethodPatch, userPath(created.ID), map[string]any{"password": newPassword})
		requireStatus(t, status, http.StatusOK, raw)
		assertNoSecrets(t, raw, newPassword)
		assertNoSecrets(t, raw, password)
		got := parseUser(t, dataObject(t, decode(t, raw)))
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, newEmail, got.Email, "email must be unchanged")
		assert.True(t, created.CreatedAt.Equal(got.CreatedAt), "created_at changed")
	})

	t.Run("delete account", func(t *testing.T) {
		status, raw := doRaw(t, http.MethodDelete, userPath(created.ID), nil, "")
		assert.Equal(t, http.StatusNoContent, status, "body: %s", raw)
		assert.Empty(t, raw)
	})

	t.Run("delete again is not found", func(t *testing.T) {
		status, body := do(t, http.MethodDelete, userPath(created.ID), nil)
		requireStatus(t, status, http.StatusNotFound, body)
		assertError(t, body, "not_found")
	})

	t.Run("fetch after delete is not found", func(t *testing.T) {
		status, body := do(t, http.MethodGet, userPath(created.ID), nil)
		requireStatus(t, status, http.StatusNotFound, body)
		assertError(t, body, "not_found")
	})
}

func TestUser_EmailIsNormalized(t *testing.T) {
	t.Parallel()

	t.Run("sign up with padded mixed-case email", func(t *testing.T) {
		local := strings.TrimSuffix(uniqueEmail(t), "@example.com")
		u := signUp(t, "  "+strings.ToUpper(local)+"@Example.COM ", "s3cret-passw0rd")
		assert.Equal(t, local+"@example.com", u.Email)
	})

	t.Run("change to padded mixed-case email", func(t *testing.T) {
		u := signUp(t, uniqueEmail(t), "s3cret-passw0rd")
		want := uniqueEmail(t)
		status, body := do(t, http.MethodPatch, userPath(u.ID), map[string]any{"email": "  " + strings.ToUpper(want) + " "})
		requireStatus(t, status, http.StatusOK, body)
		assert.Equal(t, want, parseUser(t, dataObject(t, body)).Email)
	})
}

func TestUser_SignUpValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		body     map[string]any
		wantCode string
	}{
		{"missing email", map[string]any{"password": "s3cret-passw0rd"}, "invalid_request"},
		{"invalid email format", map[string]any{"email": "not-an-email", "password": "s3cret-passw0rd"}, "invalid_request"},
		{"missing password", map[string]any{"email": uniqueEmail(t)}, "invalid_request"},
		{"password shorter than 8 characters", map[string]any{"email": uniqueEmail(t), "password": "1234567"}, "invalid_request"},
		{"password longer than 72 characters", map[string]any{"email": uniqueEmail(t), "password": strings.Repeat("a", 73)}, "invalid_request"},
		{"password within 72 characters but over 72 bytes", map[string]any{"email": uniqueEmail(t), "password": strings.Repeat("é", 40)}, "validation_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := do(t, http.MethodPost, usersPath, tc.body)
			requireStatus(t, status, http.StatusBadRequest, body)
			assertError(t, body, tc.wantCode)
		})
	}

	t.Run("malformed JSON body", func(t *testing.T) {
		status, raw := doRaw(t, http.MethodPost, usersPath, []byte(`{"email": "x@example.com",`), "application/json")
		requireStatus(t, status, http.StatusBadRequest, raw)
		assertError(t, decode(t, raw), "invalid_request")
	})

	t.Run("valid JSON without Content-Type", func(t *testing.T) {
		b := []byte(`{"email":"` + uniqueEmail(t) + `","password":"s3cret-passw0rd"}`)
		status, raw := doRaw(t, http.MethodPost, usersPath, b, "")
		requireStatus(t, status, http.StatusBadRequest, raw)
		assertError(t, decode(t, raw), "invalid_request")
	})
}

func TestUser_EmailIsUnique(t *testing.T) {
	t.Parallel()
	email := uniqueEmail(t)
	signUp(t, email, "s3cret-passw0rd")

	t.Run("sign up again with the same email", func(t *testing.T) {
		status, body := do(t, http.MethodPost, usersPath, map[string]any{"email": email, "password": "other-passw0rd"})
		requireStatus(t, status, http.StatusConflict, body)
		assert.Equal(t, "conflict: email already taken", assertError(t, body, "conflict"))
	})

	t.Run("sign up again with the same email in different case", func(t *testing.T) {
		status, body := do(t, http.MethodPost, usersPath, map[string]any{"email": strings.ToUpper(email), "password": "other-passw0rd"})
		requireStatus(t, status, http.StatusConflict, body)
		assert.Equal(t, "conflict: email already taken", assertError(t, body, "conflict"))
	})

	t.Run("sign up again with the same email padded with whitespace", func(t *testing.T) {
		status, body := do(t, http.MethodPost, usersPath, map[string]any{"email": " " + email + "  ", "password": "other-passw0rd"})
		requireStatus(t, status, http.StatusConflict, body)
		assert.Equal(t, "conflict: email already taken", assertError(t, body, "conflict"))
	})

	t.Run("another user cannot change to a taken email", func(t *testing.T) {
		other := signUp(t, uniqueEmail(t), "s3cret-passw0rd")
		status, body := do(t, http.MethodPatch, userPath(other.ID), map[string]any{"email": strings.ToUpper(email)})
		requireStatus(t, status, http.StatusConflict, body)
		assertError(t, body, "conflict")
	})
}

func TestUser_UpdateValidation(t *testing.T) {
	t.Parallel()
	u := signUp(t, uniqueEmail(t), "s3cret-passw0rd")

	t.Run("empty update is rejected with JSON field names", func(t *testing.T) {
		status, body := do(t, http.MethodPatch, userPath(u.ID), map[string]any{})
		requireStatus(t, status, http.StatusBadRequest, body)
		msg := assertError(t, body, "invalid_request")
		assert.True(t, strings.Contains(msg, "email") || strings.Contains(msg, "password"), "message should name email/password: %q", msg)
		assert.NotContains(t, msg, "Email", "message uses a Go field name")
		assert.NotContains(t, msg, "Password", "message uses a Go field name")
	})

	t.Run("update an unknown user", func(t *testing.T) {
		status, body := do(t, http.MethodPatch, userPath(uuid.NewString()), map[string]any{"email": uniqueEmail(t)})
		requireStatus(t, status, http.StatusNotFound, body)
		assertError(t, body, "not_found")
	})
}

func TestUser_LookupErrors(t *testing.T) {
	t.Parallel()

	t.Run("fetch an unknown user", func(t *testing.T) {
		status, body := do(t, http.MethodGet, userPath(uuid.NewString()), nil)
		requireStatus(t, status, http.StatusNotFound, body)
		assert.Equal(t, "user not found", assertError(t, body, "not_found"))
	})

	t.Run("fetch with a malformed id", func(t *testing.T) {
		status, body := do(t, http.MethodGet, userPath("not-a-uuid"), nil)
		requireStatus(t, status, http.StatusBadRequest, body)
		assertError(t, body, "")
	})

	t.Run("delete an unknown user", func(t *testing.T) {
		status, body := do(t, http.MethodDelete, userPath(uuid.NewString()), nil)
		requireStatus(t, status, http.StatusNotFound, body)
		assertError(t, body, "not_found")
	})
}
