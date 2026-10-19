---
paths:
  - "**/*_test.go"
  - "test/**"
---

# Testing rules

| Layer | Kind | How | Build tag |
|---|---|---|---|
| `domain` | unit, **required** for every constructor/method with a rule | pure table-driven tests, fixed `now` | — |
| `usecase` | unit, required | hand-written fakes for the ports, in the `_test.go` file | — |
| `handler` | unit, required for each endpoint | `fiber.New(handler.FiberConfig())` + `app.Test(httptest.NewRequest(...))` with a fake usecase; assert status + JSON body | — |
| `handler/errors.go` | unit, required | one case per row of the error table | — |
| `router` | unit, required | table test: every route is registered (a request to it does not return 404/405). Build `Handlers` from real handlers with fake usecases, never nil pointers | — |
| `service` | unit, when it has logic | error translation, defaults; no network (use `httptest.Server` for HTTP clients) | — |
| `repository` | integration, required for each query | real Postgres, migrations applied, tables truncated per test | `integration` |
| `test/e2e/` | end-to-end, **required and written first** (`tdd.md`) | real binary + throwaway Postgres container, HTTP only (`e2e.md`) | `e2e` |

- `go test ./...` must pass with no Docker and no DB. Anything needing infra goes behind a build tag.
- Write domain tests first. They are the spec of the business rules.
- Use table-driven tests with `t.Run(tc.name, ...)`. Use `github.com/stretchr/testify/require` for assertions.
- No mock generators. Fakes are small structs with function fields or in-memory maps.
- Assert error kinds with `require.ErrorIs(t, err, domain.ErrNotFound)`, never with string matching.
- Never call `time.Now()` in assertions. Inject a fixed clock, already UTC and truncated to µs like `SystemClock`.
- Repository integration tests compare timestamps after the round trip. They catch timezone/precision drift.
