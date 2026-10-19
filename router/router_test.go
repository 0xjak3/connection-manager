package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"connection_manager/handler"
)

// newTestHandlers builds real handlers without usecases. Route tests only check
// registration, so no request may reach a usecase.
func newTestHandlers() Handlers {
	return Handlers{User: handler.NewUserHandler(nil)}
}

func TestHealthz(t *testing.T) {
	app := New(newTestHandlers())

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"status":"ok"}`, string(body))
}

func TestRoutesAreRegistered(t *testing.T) {
	want := []struct{ method, path string }{
		{http.MethodGet, "/healthz"},
		{http.MethodPost, "/api/v1/users"},
		{http.MethodGet, "/api/v1/users"},
		{http.MethodGet, "/api/v1/users/:id"},
		{http.MethodPatch, "/api/v1/users/:id"},
		{http.MethodDelete, "/api/v1/users/:id"},
	}

	got := map[string]bool{}
	for _, r := range New(newTestHandlers()).GetRoutes(true) {
		got[r.Method+" "+r.Path] = true
	}
	for _, w := range want {
		require.True(t, got[w.method+" "+w.path], "route not registered: %s %s", w.method, w.path)
	}
}
