---
paths:
  - "test/**"
---

# E2E test rules

E2E tests are the closest thing to a real user. They see the system only as an HTTP API, exactly as a client app would.

## Environment: a fresh instance per run

`test/e2e/main_test.go` has a `TestMain` that, on every `go test` run:

1. Starts a **throwaway Postgres container** with testcontainers-go (`github.com/testcontainers/testcontainers-go/modules/postgres`), using the same image as `docker-compose.yml`. It never uses the dev or test DB from docker-compose, and never a fixed host port.
2. Applies `db/migrations` with the golang-migrate library (`github.com/golang-migrate/migrate/v4`, `pgx/v5` driver). This is the same mechanism as `make migrate-up`, so the migrations themselves are tested.
3. Builds the real binary (`go build -o <tmp>/api ./cmd/api`) and starts it as a subprocess, with `DATABASE_URL` pointing at the container and `HTTP_ADDR` on a free port. It waits until `GET /healthz` returns 200 (with a timeout), and pipes the server log to the test output.
4. After the tests, stops the server with SIGTERM (graceful shutdown) and terminates the container.

The production entry point and wiring run exactly as in deployment. Nothing is constructed in-process.

## Talking to the system

- **Everything goes through the HTTP API.** Create, read, change and delete data with the same requests a user would send.
- **Seed only when no API exists.** If a scenario needs state that no endpoint can create yet (e.g. an admin-only flag), insert it with SQL through the container's connection, in `test/e2e/seed_test.go`. Every seed function has a comment naming the missing endpoint, and it is deleted once that endpoint exists. Never seed something an API can create.
- **Never read the database to assert.** Verify through the API (e.g. after DELETE, `GET` returns 404). If the only way to observe a behaviour is the DB, ask whether the API is missing something.
- **Black box:** `test/e2e` imports **no** project packages (`connection_manager/...`, including the generated `dto`). Request and response shapes are written as local structs or `map[string]any` in the test. The hook enforces this.
- **Contract check on every call:** the HTTP helper validates every response against `api/openapi.gen.yaml` with kin-openapi (`.claude/rules/openapi.md` → Contract testing). Scenarios assert behaviour; the helper asserts the contract.

## Writing scenarios

- Write each scenario as a **user journey** in the user's words: `TestUser_SignUpThenUpdateEmail`, and steps with `t.Run("create user", ...)`, `t.Run("fetch it back", ...)`.
- Cover the happy path, then the errors a real client hits: validation (400 + `code`), not found (404), conflict (409), malformed or missing body.
- Assert **the full contract**: status code, the JSON body fields that matter, the error `code`, and fields that must NOT appear (e.g. no `password`/`password_hash` anywhere in user responses).
- **Isolation without cleanup:** each test creates its own data with unique values (e.g. `uniqueEmail(t)` = `e2e+<random>@example.com`). Never depend on another test's data or on an empty DB. For list endpoints, assert "contains what I created", not exact counts. Mark tests `t.Parallel()`.
- Keep a tiny client helper (`test/e2e/client_test.go`): `do(t, method, path, body) (status int, body map[string]any)`. No retries or sleeps inside scenarios.

## Files and running

```
test/e2e/main_test.go      TestMain: container + migrations + binary + healthz wait + load api/openapi.gen.yaml
test/e2e/client_test.go    HTTP helper (validates responses against the spec), uniqueEmail, assertions
test/e2e/seed_test.go      only if a seed is unavoidable (see above)
test/e2e/<feature>_test.go scenarios per feature (users_test.go, ...)
```

- Every file has `//go:build e2e`. `go test ./...` without the tag never needs Docker.
- Run with `make test-e2e` (`go test -tags=e2e -count=1 ./test/e2e/...`). Needs a Docker daemon, and nothing else.
