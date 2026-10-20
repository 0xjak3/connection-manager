---
paths:
  - "handler/**"
  - "dto/**"
  - "router/**"
  - "cmd/**"
---

# Handler, dto and router rules (Fiber v3)

## dto (`dto/dto.gen.go`, generated from the spec in `api/`)

- dto structs are **generated** by oapi-codegen from the spec in `api/` (`.claude/rules/openapi.md`). To change a request or response, change the spec and run `make generate`. Never edit `dto/dto.gen.go`; the hook blocks it.
- Envelopes are concrete per resource (`UserResponse{Data User}`, `UserListResponse{Data []User}`) plus `ErrorResponse{Error ErrorBody}`. There is no generic envelope.
- Validate tags come from `x-oapi-codegen-extra-tags` in the spec and must match the schema constraints.
- PATCH requests are pointer fields (`*string`, `omitempty`) with `omitnil` in the validate tag. Cross-field rules use validator tags (`required_without=Password`). If a rule can't be expressed in tags, register a custom validation in `handler/validator.go`.
- A value that needs trimming before format validation uses a custom tag registered in `handler/validator.go` (e.g. `trimmed_email`). The dto value is passed on unchanged and the domain still normalizes it.
- JSON keys are the snake_case of the Go field name, always (the generator uses `ToCamelCaseWithInitialisms`, so `id` → `ID`, `created_at` → `CreatedAt`). `validationMessage` relies on this to translate field names in tag params (`required_without=Password` → `password`).
- `max`/`min` count characters (runes). Byte limits (bcrypt's 72 bytes) are enforced in the service/domain and come back as `domain.ErrValidation`.

## handler (`handler/<aggregate>.go`)

```go
type connectionUsecase interface {
	Create(ctx context.Context, in usecase.CreateConnectionInput) (domain.Connection, error)
}

type ConnectionHandler struct{ uc connectionUsecase }

func NewConnectionHandler(uc connectionUsecase) *ConnectionHandler {
	return &ConnectionHandler{uc: uc}
}

func (h *ConnectionHandler) Create(c fiber.Ctx) error {
	var req dto.CreateConnectionRequest
	if err := c.Bind().Body(&req); err != nil { // runs StructValidator too
		return err
	}
	conn, err := h.uc.Create(c.Context(), usecase.CreateConnectionInput{Name: req.Name})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(dto.ConnectionResponse{Data: toConnection(conn)})
}
```

- Steps: bind + validate the dto → map it to a usecase input → call the usecase → map the returned domain to a dto → respond. Nothing else.
- Fiber v3 signatures: handlers are `func(c fiber.Ctx) error` (`Ctx` is an interface, not a pointer). Pass `c.Context()` (a `context.Context`) to the usecase.
- Mapping domain → dto uses private functions in `handler/<aggregate>_mapper.go`, e.g. `toConnection(domain.Connection) dto.Connection`.
- Parse path params (UUIDs etc.) in the handler. If one is invalid, return `fiber.NewError(fiber.StatusBadRequest, "invalid id")`.
- Handlers only pick the success status. On failure they `return err` and let the central error handler map it.
- Never call repository/service, never hold business rules, never return domain structs as JSON directly.

## Fiber config (`handler/config.go`) and validator (`handler/validator.go`)

- `handler.FiberConfig() fiber.Config` returns `ErrorHandler` + `StructValidator` (the go-playground adapter in `handler/validator.go`, reporting JSON field names). It is the single source of the app config: `router.New` uses it, and handler tests build `fiber.New(handler.FiberConfig())`. Never copy config fields into tests.
- Validation messages use JSON names, including in tag params that reference other fields (`required_without=Password` → `password`). Translate them in `validationMessage`.

## Error handler (`handler/errors.go`)

A single `func ErrorHandler(c fiber.Ctx, err error) error`, wired via `FiberConfig()`. Cases are checked in this order:

| Error | Status | `code` |
|---|---|---|
| `validator.ValidationErrors` | 400 | `invalid_request` |
| `*fiber.BindError` (malformed body) | 400 | `invalid_request` |
| `fiber.ErrUnprocessableEntity` from Bind (missing/unsupported `Content-Type`) | 400 | `invalid_request` |
| other `*fiber.Error` | its code | `http_error` |
| `domain.ErrValidation` | 400 | `validation_error` |
| `domain.ErrNotFound` | 404 | `not_found` |
| `domain.ErrConflict` | 409 | `conflict` |
| `domain.ErrInvalidState` | 409 | `invalid_state` |
| `domain.ErrForbidden` | 403 | `forbidden` |
| anything else | 500 | `internal_error` (log the error; send a generic message) |

The body is always `dto.ErrorResponse`. The two 400 codes mean different things to clients: `invalid_request` means the request shape is wrong (fix the request format), and `validation_error` means the business rules rejected the request.

## router (`router/router.go`)

- `func New(h Handlers) *fiber.App` creates the app with `fiber.New(handler.FiberConfig())`, adds recover + request-log middleware, and registers every route in one readable table.
- `GET /healthz` at the root. Business routes go under `/api/v1`, using REST resource naming (`/api/v1/connections/:id`).
- `Handlers` is a struct holding every handler, built in `cmd/api/main.go`.

## cmd/api/main.go

Load config (`util`) → `pgxpool.New` → repositories → services → usecases → handlers → `router.New` → `app.Listen`, with graceful shutdown on SIGINT/SIGTERM (`app.ShutdownWithContext`, then `pool.Close()`). Wiring only, no logic.
