# connection_manager

Go HTTP API. Purpose and feature set are **not defined yet**. Do not invent entities, fields, states or business rules; ask when a task doesn't specify them.

## Stack

- Go 1.26, module `connection_manager`
- HTTP: Fiber v3 (`github.com/gofiber/fiber/v3`)
- DB: PostgreSQL via `github.com/jackc/pgx/v5` (`pgxpool`)
- Queries: sqlc (`sql_package: pgx/v5`), generated into `repository/sqlcgen`
- Migrations: golang-migrate file format (`000001_name.up.sql` / `000001_name.down.sql`)
- Validation: `github.com/go-playground/validator/v10`, registered as Fiber's `StructValidator`
- IDs: UUID v7 (`github.com/google/uuid`)

## Layout

```
cmd/api/main.go      composition root: config → pgxpool → repositories → services → usecases → handlers → router → listen
router/              Fiber app setup (error handler, middleware) + route table
handler/             HTTP: bind/validate dto → call usecase → map domain → dto response
dto/                 request/response structs GENERATED from openapi.yaml (dto.gen.go), never edit by hand
usecase/             application logic: orchestrates domain + repositories + services. Declares its ports (interfaces)
domain/              entities, value objects, domain errors, business rules as methods
service/             adapters to 3rd-party / external capabilities (HTTP APIs, email, clock, crypto, …)
repository/          persistence: implements usecase ports, maps sqlcgen rows ↔ domain
repository/sqlcgen/  sqlc output. GENERATED, never edit by hand
db/migrations/       schema (also sqlc's schema input)
db/queries/          sqlc query files, one per table/aggregate
util/                generic helpers with no business meaning (config, logger)
test/                end-to-end API tests (build tag `e2e`)
sqlc.yaml
openapi.yaml         API contract (spec-first): source of dto and of e2e contract checks
oapi-codegen.yaml    dto generator config
```

## Request flow

```
HTTP ─▶ router ─▶ handler ──dto → usecase input──▶ usecase ──▶ domain (constructors / methods)
                                                     ├──────▶ repository ─▶ sqlcgen ─▶ Postgres
                                                     └──────▶ service ─▶ 3rd party
HTTP ◀── handler ◀──domain → dto── usecase (returns domain)
```

## Golden rules

1. Repository returns **domain** entities, never sqlc types.
2. Business rules live on **domain** entities (constructors, methods). Usecase orchestrates; it does not decide.
3. Usecase returns **domain**. Handler maps domain → dto. Usecase never sees dto or Fiber.
4. Interfaces are declared by their consumer (`usecase/ports.go`, small interfaces in handler), and implemented by repository/service.
5. Errors: repository/service translate infra errors into domain errors; the router's error handler is the only place that maps domain errors to HTTP status.
6. `context.Context` is the first parameter of every usecase, repository and service method.
7. **Spec-first, TDD outside in:** update `openapi.yaml` first (`.claude/rules/openapi.md`), write a failing e2e scenario, implement, then make it pass (`.claude/rules/tdd.md`). E2E tests use the API like a real user, against a throwaway container (`.claude/rules/e2e.md`).

Per-layer details are in `.claude/rules/`: `tdd.md`, `openapi.md`, `e2e.md`, `architecture.md`, `domain.md`, `repository.md`, `usecase-service.md`, `handler-dto.md`, `testing.md`.
To add a feature end-to-end (and bootstrap the project on first use), follow `.claude/skills/new-feature/SKILL.md`.

## Commands (run from the project root)

```
make db-up              # docker compose (project `connection_manager`): dev Postgres 17 on 127.0.0.1:5440, test Postgres on 127.0.0.1:5441
make migrate-up         # apply db/migrations to the dev DB (migrate CLI if installed, else the migrate/migrate image)
make migrate-down       # roll back the last migration on the dev DB
make generate           # sqlc generate (after changing db/queries or db/migrations)
make test               # go test ./...  (unit tests, no DB needed)
make test-integration   # starts the test DB, then go test -tags=integration ./repository/...
make run                # go run ./cmd/api against the dev DB (HTTP_ADDR defaults to :8080)
make db-down            # docker compose down (dev data volume is kept; test DB is tmpfs)
make lint-layers        # layer import check over every .go file
make test-e2e           # e2e: throwaway Postgres container + real binary, HTTP only (needs Docker)
make check              # lint-layers + vet + build + unit tests + sqlc vet (Definition of Done, no DB)

go build ./...
go vet ./...
go test -tags=e2e ./test/...                   # needs running stack
sqlc vet
```

Env: `DATABASE_URL` (required by the API; Makefile default `postgres://cm:cm@localhost:5440/connection_manager?sslmode=disable`), `HTTP_ADDR`, and `TEST_DATABASE_URL` for integration tests (default `postgres://cm:cm@localhost:5441/connection_manager_test?sslmode=disable`). Integration tests drop and recreate the `public` schema of the test DB and apply every `*.up.sql` directly. Never point `TEST_DATABASE_URL` at the dev DB.

## Definition of Done

- [ ] E2E scenarios for the change exist, were seen failing first (red), and `make test-e2e` now passes (green)
- [ ] `make check` passes (layer check, vet, build, unit tests, sqlc vet; gofmt runs automatically on every edit)
- [ ] If repository changed: `make test-integration` passes
- [ ] If SQL changed: `sqlc generate` was run and the generated code is up to date
- [ ] Every new migration has both up and down files
- [ ] Tests exist as required by `.claude/rules/testing.md`
- [ ] The post-edit hook reports no layer violations, and the golden rules hold

## Harness

`.claude/hooks/post-edit.sh` runs after every Edit/Write of a `.go` file. It runs gofmt, checks import boundaries per layer, and blocks hand edits to `repository/sqlcgen/`. If it blocks you, fix the design. Do not bypass the hook.
