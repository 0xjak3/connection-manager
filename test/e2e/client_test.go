//go:build e2e

package e2e

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// do sends body (marshalled as JSON, with Content-Type: application/json, unless nil)
// and returns the status and the decoded JSON body (nil when the response body is empty).
func do(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()
	status, raw := doJSON(t, method, path, body)
	return status, decode(t, raw)
}

// doJSON is like do but returns the raw response body.
func doJSON(t *testing.T, method, path string, body any) (int, []byte) {
	t.Helper()
	if body == nil {
		return doRaw(t, method, path, nil, "")
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	return doRaw(t, method, path, b, "application/json")
}

// doRaw sends body as-is. An empty contentType sends no Content-Type header.
func doRaw(t *testing.T, method, path string, body []byte, contentType string) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, baseURL+path, r)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, path, err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body of %s %s: %v", method, path, err)
	}
	return resp.StatusCode, raw
}

func decode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("response is not a JSON object: %v\nbody: %s", err, raw)
	}
	return m
}

// uniqueEmail returns e2e+<random>@example.com.
func uniqueEmail(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("random: %v", err)
	}
	return "e2e+" + hex.EncodeToString(b) + "@example.com"
}

// requireStatus fails the test immediately when the status is not want.
func requireStatus(t *testing.T, got, want int, body any) {
	t.Helper()
	if got != want {
		if b, ok := body.([]byte); ok {
			body = string(b)
		}
		t.Fatalf("status = %d, want %d; body: %v", got, want, body)
	}
}

// dataObject returns body["data"] as an object.
func dataObject(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	d, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf(`body has no "data" object: %v`, body)
	}
	return d
}

// assertError checks the error envelope and code, and returns the message.
func assertError(t *testing.T, body map[string]any, wantCode string) string {
	t.Helper()
	e, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf(`body has no "error" object: %v`, body)
	}
	code, _ := e["code"].(string)
	msg, ok := e["message"].(string)
	if !ok {
		t.Errorf(`error has no string "message": %v`, e)
	}
	if wantCode != "" && code != wantCode {
		t.Errorf("error.code = %q, want %q (message: %q)", code, wantCode, msg)
	}
	if wantCode == "" && code == "" {
		t.Errorf("error.code is empty: %v", e)
	}
	return msg
}

// assertNoSecrets checks the raw body exposes neither password keys nor the plaintext password.
func assertNoSecrets(t *testing.T, raw []byte, plaintext string) {
	t.Helper()
	s := string(raw)
	if strings.Contains(s, `"password"`) || strings.Contains(s, `"password_hash"`) {
		t.Errorf("response exposes a password key: %s", s)
	}
	if plaintext != "" && strings.Contains(s, plaintext) {
		t.Errorf("response contains the plaintext password: %s", s)
	}
}

// user is the parsed user object.
type user struct {
	ID        string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// parseUser checks the user object's shape and returns it.
func parseUser(t *testing.T, obj map[string]any) user {
	t.Helper()
	var u user
	var ok bool
	if u.ID, ok = obj["id"].(string); !ok {
		t.Fatalf(`user has no string "id": %v`, obj)
	}
	if _, err := uuid.Parse(u.ID); err != nil {
		t.Errorf("user id %q is not a uuid: %v", u.ID, err)
	}
	if u.Email, ok = obj["email"].(string); !ok {
		t.Errorf(`user has no string "email": %v`, obj)
	}
	u.CreatedAt = parseUTC(t, obj, "created_at")
	u.UpdatedAt = parseUTC(t, obj, "updated_at")
	for _, k := range []string{"password", "password_hash"} {
		if _, found := obj[k]; found {
			t.Errorf("user exposes %q: %v", k, obj)
		}
	}
	return u
}

func parseUTC(t *testing.T, obj map[string]any, key string) time.Time {
	t.Helper()
	s, ok := obj[key].(string)
	if !ok {
		t.Errorf("user has no string %q: %v", key, obj)
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Errorf("%s = %q is not RFC 3339: %v", key, s, err)
	}
	if !strings.HasSuffix(s, "Z") {
		t.Errorf("%s = %q is not UTC with a Z suffix", key, s)
	}
	return ts
}

// signUp creates a user and returns it; it fails the test unless the API answers 201.
func signUp(t *testing.T, email, password string) user {
	t.Helper()
	status, raw := doJSON(t, http.MethodPost, "/api/v1/users", map[string]any{"email": email, "password": password})
	requireStatus(t, status, http.StatusCreated, raw)
	assertNoSecrets(t, raw, password)
	return parseUser(t, dataObject(t, decode(t, raw)))
}
