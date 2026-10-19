# Architecture rules

## Dependency rules

Arrows point inward to `domain`. `domain` depends on nothing in the project.

| Layer | May import (project packages) | Must NOT import |
|---|---|---|
| `domain` | — (stdlib + small pure libs like `google/uuid`) | any other project package, fiber, pgx, sqlcgen, `database/sql`, `net/http` |
| `dto` | — (stdlib only) | any project package |
| `handler` | `usecase`, `domain`, `dto` | `repository`, `service`, `router`, `util`, `sqlcgen`, pgx |
| `router` | `handler`, `dto` | `repository`, `service`, `usecase`, `util`, pgx |
| `usecase` | `domain` | `handler`, `dto`, `router`, `repository`, `service`, `util`, fiber, pgx |
| `service` | `domain` | `handler`, `dto`, `router`, `usecase`, `repository`, `util`, fiber |
| `repository` | `domain`, `repository/sqlcgen` | `handler`, `dto`, `router`, `usecase`, `service`, `util`, fiber |
| `util` | — | any other project package |
| `cmd` | everything (composition root) | — |
| `test/e2e` | — (black box, HTTP only) | any project package |

`.claude/hooks/post-edit.sh` enforces this table on every edit; `make lint-layers` runs it over the whole tree.

`util` holds process-level helpers (config loading) and is imported **only by `cmd`**. Values a layer needs (e.g. bcrypt cost, upstream URL) are passed in through constructor config structs. Logging uses stdlib `log/slog` directly. There is no logger wrapper.

## Wiring

- Usecase declares the interfaces it needs in `usecase/ports.go` (e.g. `ConnectionRepository`, `Clock`). Repository and service structs satisfy them implicitly. `cmd/api/main.go` wires concrete types into constructors.
- Handlers depend on small interfaces declared in the handler package (only the usecase methods they call), so handler tests can use fakes.
- No DI framework and no globals. Everything is passed through constructors: `NewXxx(deps...) *Xxx`.

## Naming and files

- One aggregate per file in each layer: `domain/connection.go`, `repository/connection.go`, `usecase/connection.go`, `handler/connection.go`, `dto/connection.go`, `db/queries/connections.sql`.
- Types carry the layer name: `usecase.ConnectionUsecase`, `repository.ConnectionRepository`, `handler.ConnectionHandler`, `service.XxxService`.
- Keep layer packages flat. Only split a layer into sub-packages when it grows past ~10 files, and ask first.

## Errors

- **All** domain errors live in `domain/errors.go`: the kind sentinels (`ErrValidation`, `ErrNotFound`, `ErrConflict`, `ErrInvalidState`, `ErrForbidden`) and the entity-specific errors that wrap a kind. Entity errors are grouped under a comment per aggregate. Word them so `err.Error()` reads naturally as an API message: `ErrUserNotFound = fmt.Errorf("user %w", ErrNotFound)` gives "user not found", and `ErrEmailTaken = fmt.Errorf("%w: email already taken", ErrConflict)` gives "conflict: email already taken".
- Lower layers wrap with `%w` to add context. Never compare error strings. Use `errors.Is` / `errors.As`.
- Only the router's `ErrorHandler` (implemented in `handler/errors.go`) turns errors into HTTP status + body.

## Time

- Every timestamp in the system is **UTC with microsecond precision** (Postgres `timestamptz` precision).
- `service.SystemClock.Now()` returns `time.Now().UTC().Truncate(time.Microsecond)`. This way an entity returned right after Create equals the same entity loaded from the DB.
- The repository normalizes times read from rows with `.UTC()`, because pgx returns them in the process's local zone.
- The domain never calls `time.Now()`. It receives `now` as a parameter.

## Concurrency

Not decided yet. Updates are load → modify → write the full row, so concurrent updates are last-write-wins. If a use case can't tolerate that, raise it before implementing (the candidate is an optimistic check on `updated_at` that returns `ErrConflict`).

## Don't build ahead

Add something (transactions, pagination, caching, auth middleware, sub-packages) when a real task needs it, not before. When a task needs one for the first time, design it once here and record the decision in the relevant rules file.
