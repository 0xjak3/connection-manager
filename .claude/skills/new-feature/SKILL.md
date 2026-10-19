---
name: new-feature
description: Add a feature to connection_manager end-to-end (api spec → e2e → migration → sqlc → domain → repository → usecase → service → dto/handler → route → tests), bootstrapping the project first if it is still empty. Use when adding an endpoint, entity or use case.
---

# New feature

Work through the steps in order, following the TDD loop in `.claude/rules/tdd.md`. Each step must leave the project in a building state (`go build ./...`).

## 0. Confirm the spec

Write down what you will build before touching code: entity fields, invariants, state transitions, endpoints (method, path, request, response, error cases → domain error kinds). If anything is missing, stop and ask. Don't invent business rules.

## 0.4. Contract: the spec in api/

Add `api/paths/<resource>.yaml` and `api/schemas/<resource>.yaml`, and register them in `api/openapi.yaml`, per `.claude/rules/openapi.md` (schemas named like the dto types, error codes listed, validate tags = schema constraints). Run `make generate`.

## 0.5. Red: e2e scenarios first

Write the scenarios for this feature in `test/e2e/<feature>_test.go` (`.claude/rules/e2e.md`). If `test/e2e/main_test.go` doesn't exist yet, build the harness first. Run `make test-e2e` and confirm the scenarios fail for the right reason (404/405, wrong status or body). Keep that output. Don't edit these scenarios during steps 2–8.

## 1. Bootstrap (only if not done yet: no `sqlc.yaml`, `router/`, or `handler/errors.go`; already done in this repo)

1. `go get github.com/gofiber/fiber/v3 github.com/jackc/pgx/v5 github.com/google/uuid github.com/go-playground/validator/v10 github.com/stretchr/testify`
2. `sqlc.yaml` from `.claude/rules/repository.md`. Create `db/migrations/` and `db/queries/`.
3. `domain/errors.go` (kind sentinels), `service/clock.go` (`SystemClock`), `repository/errors.go` (`translateErr`).
4. `api/` spec + `tools/openapibundle` + `oapi-codegen.yaml` (dto generated), `handler/errors.go` (`ErrorHandler`), `handler/validator.go` (adapter implementing `fiber.StructValidator`), `handler/config.go` (`FiberConfig()`).
5. `util/config.go`: env `HTTP_ADDR` (default `:8080`) and `DATABASE_URL` (required). Only `cmd` imports `util`.
6. `router/router.go` with `GET /healthz`, and `cmd/api/main.go` with wiring + graceful shutdown.
7. `docker-compose.yml` (Postgres for dev, plus a separate DB for integration tests) and a `Makefile` (`generate`, `migrate-up`, `test`, `test-integration`, `run`). Update the Commands section in `.claude/CLAUDE.md`.
8. Tests: `handler/errors_test.go` for the error mapping table, and `/healthz` via `app.Test`.

## 2. Domain (unit test first: red → green)

`domain/<aggregate>_test.go` (table-driven: valid and invalid constructor input, allowed and forbidden transitions), then `domain/<aggregate>.go`. Add entity-specific errors to `domain/errors.go`. Add value-object parse functions if the usecase must validate before expensive work. → `.claude/rules/domain.md`

## 3. Schema and queries (if persistence changes)

New migration up + down → `db/queries/<table>.sql` → `sqlc generate`. → `.claude/rules/repository.md`

## 4. Repository

`repository/<aggregate>.go` with private row ↔ domain mappers and `translateErr`. Add an integration test (`//go:build integration`).

## 5. Usecase

Add the ports to `usecase/ports.go`, then write `usecase/<aggregate>.go` with input structs. Unit tests use fakes, covering: happy path, a domain error passed through, and a repository error passed through. → `.claude/rules/usecase-service.md`

## 6. Service (only if an external capability is needed)

`service/<name>.go` implementing a usecase port. Domain/primitive types only at its boundary.

## 7. handler (dto already generated in 0.4)

`handler/<aggregate>.go` with a consumer interface, and `handler/<aggregate>_mapper.go`. Handler tests build `fiber.New(handler.FiberConfig())` and use `app.Test` with a fake usecase: success, validation failure (400), and one domain error mapping. → `.claude/rules/handler-dto.md`

## 8. Route + wiring

Register routes in `router/router.go` under `/api/v1` and add them to the router's route-table test. Wire concrete types in `cmd/api/main.go`.

## 9. Green + Definition of Done

Run `make test-e2e` (must pass now), `make check` (plus `make test-integration` if repository changed) and every other item in the Definition of Done section of `.claude/CLAUDE.md`, and report the actual results, including the red e2e output from step 0.5 and the green one from step 9.
