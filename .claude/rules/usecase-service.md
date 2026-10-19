---
paths:
  - "usecase/**"
  - "service/**"
---

# Usecase and service rules

## Usecase

Usecase is the application script: it says *what happens in what order*. It doesn't hold the rules.

```go
// usecase/ports.go
type ConnectionRepository interface {
	Create(ctx context.Context, c domain.Connection) error
	GetByID(ctx context.Context, id uuid.UUID) (domain.Connection, error)
	Update(ctx context.Context, c domain.Connection) error
}

type Clock interface{ Now() time.Time }
```

```go
// usecase/connection.go
type ConnectionUsecase struct {
	repo  ConnectionRepository
	clock Clock
}

func NewConnectionUsecase(repo ConnectionRepository, clock Clock) *ConnectionUsecase {
	return &ConnectionUsecase{repo: repo, clock: clock}
}

type CreateConnectionInput struct{ Name string }

func (u *ConnectionUsecase) Create(ctx context.Context, in CreateConnectionInput) (domain.Connection, error) {
	conn, err := domain.NewConnection(in.Name, u.clock.Now())
	if err != nil {
		return domain.Connection{}, err
	}
	if err := u.repo.Create(ctx, conn); err != nil {
		return domain.Connection{}, err
	}
	return conn, nil
}
```

- Method shape: `(ctx context.Context, in XxxInput) (domain.Xxx, error)`. Plain parameters are fine for one or two values (`GetByID(ctx, id)`).
- Input structs live in `usecase` and have no tags. They are not dto.
- Return domain values: `domain.Xxx`, `[]domain.Xxx`, or `error` only.
- Typical body: load → call domain constructor/method → persist → side effects via services → return domain.
- Allowed here: ordering steps, choosing which port to call, transaction boundaries, passing `now`/IDs into the domain, combining results from several ports.
- Not allowed here, so move it to `domain`: conditionals on entity fields, computing derived values, business validation, state changes by assigning fields.
- Input *shape* validation (required, format, length) is done by dto `validate` tags in the handler. *Business* validation is done by domain constructors and methods.
- Don't rely on the handler for correctness: if an input makes the use case meaningless on its own (e.g. an update with no fields set), return `fmt.Errorf("%w: ...", domain.ErrValidation)` instead of doing a no-op write.
- Cheap before expensive: validate with domain parse functions (`domain.ParseEmail`) before calling expensive ports (password hashing, external APIs).
- Return domain errors as-is, or wrap them with `%w`. Never create HTTP-flavoured errors or reference status codes.
- No Fiber, no dto, no sqlcgen, no direct `time.Now()` (use `Clock`).

## Service

A service is an adapter for anything that isn't the database. There are two kinds:

- **External I/O**: 3rd-party HTTP APIs, email/SMS, object storage
- **Local capabilities**: password hashing, encryption, token generation, the system clock

Rules for both:

- One struct per capability, implementing a port declared in `usecase/ports.go`, in `service/<capability>.go`.
- Name it after what it is: `SystemClock`, `PasswordService`, `SendgridMailer`. The `Service` suffix is optional; the name must not repeat the package (`service.ClockService` ✗).
- Constructor: `NewXxx(cfg XxxConfig, deps...) *Xxx`. `XxxConfig` holds tunables (bcrypt cost, base URL, timeout) with documented zero-value defaults. `deps` are only what it actually needs (`*http.Client` for external I/O, nothing for bcrypt). Stateless services like `SystemClock` need no constructor.
- Inputs and outputs are domain types or primitives. SDK/HTTP response types never leak out.
- Translate failures that mean something to the business into domain errors. For example, upstream 404 → `domain.ErrNotFound`, and bcrypt `ErrPasswordTooLong` → `fmt.Errorf("%w: password must be at most 72 bytes", domain.ErrValidation)`. Wrap everything else: `fmt.Errorf("service: <op>: %w", err)`.
- `ctx`: external I/O must pass it to every call and set explicit client timeouts. CPU-bound work checks `ctx.Err()` once before starting.
- Unit-test any service that has logic (error translation, defaults). Thin pass-throughs don't need tests.

The canonical minimal service is `service/clock.go` (UTC, µs: see `architecture.md` → Time):

```go
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }
```
