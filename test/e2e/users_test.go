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
		assert.True(t, afterEmail.UpdatedAt.After(created.UpdatedAt), "updated_at %v must be after %v", afterEmail.UpdatedAt, created.UpdatedAt)
	})

	t.Run("fetch shows the new email", func(t *testing.T) {
		status, body := do(t, http.MethodGet, userPath(created.ID), nil)
		requireStatus(t, status, http.StatusOK, body)
		assert.Equal(t, newEmail, parseUser(t, dataObject(t, body)).Email)
	})

	// The new password itself is not observable through the API (there is no login
	// endpoint yet), so this step only checks the response and that nothing else changed.
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
		{"blank email", map[string]any{"email": "   ", "password": "s3cret-passw0rd"}, "invalid_request"},
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

	t.Run("valid JSON sent as text/plain", func(t *testing.T) {
		b := []byte(`{"email":"` + uniqueEmail(t) + `","password":"s3cret-passw0rd"}`)
		status, raw := doRaw(t, http.MethodPost, usersPath, b, "text/plain")
		requireStatus(t, status, http.StatusBadRequest, raw)
		assertError(t, decode(t, raw), "invalid_request")
	})

	t.Run("empty JSON body", func(t *testing.T) {
		status, raw := doRaw(t, http.MethodPost, usersPath, []byte{}, "application/json")
		requireStatus(t, status, http.StatusBadRequest, raw)
		assertError(t, decode(t, raw), "invalid_request")
	})
}

// passwordBoundaries are the longest/shortest passwords that must be accepted:
// 8 and 72 characters, and 72 bytes in UTF-8 (36 two-byte runes).
var passwordBoundaries = []struct {
	name     string
	password string
}{
	{"password of exactly 8 characters", "12345678"},
	{"password of exactly 72 ASCII characters", strings.Repeat("a", 72)},
	{"password of exactly 72 bytes", strings.Repeat("é", 36)},
}

func TestUser_PasswordLengthBoundariesAreAccepted(t *testing.T) {
	t.Parallel()

	for _, tc := range passwordBoundaries {
		t.Run("sign up with a "+tc.name, func(t *testing.T) {
			email := uniqueEmail(t)
			u := signUp(t, email, tc.password)
			assert.Equal(t, email, u.Email)
		})
	}

	for _, tc := range passwordBoundaries {
		t.Run("change to a "+tc.name, func(t *testing.T) {
			email := uniqueEmail(t)
			u := signUp(t, email, "s3cret-passw0rd")
			status, raw := doJSON(t, http.MethodPatch, userPath(u.ID), map[string]any{"password": tc.password})
			requireStatus(t, status, http.StatusOK, raw)
			assertNoSecrets(t, raw, tc.password)
			got := parseUser(t, dataObject(t, decode(t, raw)))
			assert.Equal(t, u.ID, got.ID)
			assert.Equal(t, email, got.Email)
		})
	}
}

func TestUser_ChangeEmailAndPasswordTogether(t *testing.T) {
	t.Parallel()
	u := signUp(t, uniqueEmail(t), "s3cret-passw0rd")
	newEmail := uniqueEmail(t)
	newPassword := "another-s3cret"

	t.Run("change both in one request", func(t *testing.T) {
		status, raw := doJSON(t, http.MethodPatch, userPath(u.ID), map[string]any{"email": newEmail, "password": newPassword})
		requireStatus(t, status, http.StatusOK, raw)
		assertNoSecrets(t, raw, newPassword)
		got := parseUser(t, dataObject(t, decode(t, raw)))
		assert.Equal(t, u.ID, got.ID)
		assert.Equal(t, newEmail, got.Email)
	})

	t.Run("fetch shows the new email", func(t *testing.T) {
		status, body := do(t, http.MethodGet, userPath(u.ID), nil)
		requireStatus(t, status, http.StatusOK, body)
		assert.Equal(t, newEmail, parseUser(t, dataObject(t, body)).Email)
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
		otherEmail := uniqueEmail(t)
		other := signUp(t, otherEmail, "s3cret-passw0rd")
		status, body := do(t, http.MethodPatch, userPath(other.ID), map[string]any{"email": strings.ToUpper(email)})
		requireStatus(t, status, http.StatusConflict, body)
		assert.Equal(t, "conflict: email already taken", assertError(t, body, "conflict"))

		status, body = do(t, http.MethodGet, userPath(other.ID), nil)
		requireStatus(t, status, http.StatusOK, body)
		got := parseUser(t, dataObject(t, body))
		assert.Equal(t, otherEmail, got.Email, "rejected change must leave the email unchanged")
		assert.True(t, other.UpdatedAt.Equal(got.UpdatedAt), "rejected change must leave updated_at unchanged")
	})
}

func TestUser_UpdateValidation(t *testing.T) {
	t.Parallel()
	u := signUp(t, uniqueEmail(t), "s3cret-passw0rd")

	t.Run("empty update is rejected with JSON field names", func(t *testing.T) {
		status, body := do(t, http.MethodPatch, userPath(u.ID), map[string]any{})
		requireStatus(t, status, http.StatusBadRequest, body)
		assert.Equal(t,
			"email failed required_without=password; password failed required_without=email",
			assertError(t, body, "invalid_request"))
	})

	cases := []struct {
		name     string
		body     map[string]any
		wantCode string
	}{
		{"invalid email format", map[string]any{"email": "not-an-email"}, "invalid_request"},
		{"blank email", map[string]any{"email": "   "}, "invalid_request"},
		{"password shorter than 8 characters", map[string]any{"password": "1234567"}, "invalid_request"},
		{"password longer than 72 characters", map[string]any{"password": strings.Repeat("a", 73)}, "invalid_request"},
		{"password within 72 characters but over 72 bytes", map[string]any{"password": strings.Repeat("é", 40)}, "validation_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := do(t, http.MethodPatch, userPath(u.ID), tc.body)
			requireStatus(t, status, http.StatusBadRequest, body)
			assertError(t, body, tc.wantCode)
		})
	}

	t.Run("malformed JSON body", func(t *testing.T) {
		status, raw := doRaw(t, http.MethodPatch, userPath(u.ID), []byte(`{"email": "x@example.com",`), "application/json")
		requireStatus(t, status, http.StatusBadRequest, raw)
		assertError(t, decode(t, raw), "invalid_request")
	})

	t.Run("valid JSON without Content-Type", func(t *testing.T) {
		b := []byte(`{"email":"` + uniqueEmail(t) + `"}`)
		status, raw := doRaw(t, http.MethodPatch, userPath(u.ID), b, "")
		requireStatus(t, status, http.StatusBadRequest, raw)
		assertError(t, decode(t, raw), "invalid_request")
	})

	t.Run("valid JSON sent as text/plain", func(t *testing.T) {
		b := []byte(`{"email":"` + uniqueEmail(t) + `"}`)
		status, raw := doRaw(t, http.MethodPatch, userPath(u.ID), b, "text/plain")
		requireStatus(t, status, http.StatusBadRequest, raw)
		assertError(t, decode(t, raw), "invalid_request")
	})

	t.Run("rejected updates left the user unchanged", func(t *testing.T) {
		status, body := do(t, http.MethodGet, userPath(u.ID), nil)
		requireStatus(t, status, http.StatusOK, body)
		got := parseUser(t, dataObject(t, body))
		assert.Equal(t, u.Email, got.Email)
		assert.True(t, u.UpdatedAt.Equal(got.UpdatedAt), "updated_at changed: %v -> %v", u.UpdatedAt, got.UpdatedAt)
	})

	t.Run("update an unknown user", func(t *testing.T) {
		status, body := do(t, http.MethodPatch, userPath(uuid.NewString()), map[string]any{"email": uniqueEmail(t)})
		requireStatus(t, status, http.StatusNotFound, body)
		assert.Equal(t, "user not found", assertError(t, body, "not_found"))
	})
}

func TestUser_LookupErrors(t *testing.T) {
	t.Parallel()

	t.Run("fetch an unknown user", func(t *testing.T) {
		status, body := do(t, http.MethodGet, userPath(uuid.NewString()), nil)
		requireStatus(t, status, http.StatusNotFound, body)
		assert.Equal(t, "user not found", assertError(t, body, "not_found"))
	})

	t.Run("delete an unknown user", func(t *testing.T) {
		status, body := do(t, http.MethodDelete, userPath(uuid.NewString()), nil)
		requireStatus(t, status, http.StatusNotFound, body)
		assert.Equal(t, "user not found", assertError(t, body, "not_found"))
	})
}

func TestUser_MalformedID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		method string
		body   map[string]any
	}{
		{http.MethodGet, nil},
		{http.MethodPatch, map[string]any{"email": uniqueEmail(t)}},
		{http.MethodDelete, nil},
	}
	for _, tc := range cases {
		t.Run(tc.method+" with a malformed id", func(t *testing.T) {
			status, body := do(t, tc.method, userPath("not-a-uuid"), tc.body)
			requireStatus(t, status, http.StatusBadRequest, body)
			assert.Equal(t, "invalid id", assertError(t, body, "http_error"))
		})
	}
}
