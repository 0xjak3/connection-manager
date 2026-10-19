---
paths:
  - "domain/**"
  - "usecase/**"
---

# Domain rules

The domain is where the business lives. When you write an `if` about an entity's state in another layer, it belongs here instead.

## What belongs in `domain/`

- Entities (structs with identity) and value objects (typed values, e.g. `type Status string` with `IsValid()`)
- Constructors that enforce invariants: `NewXxx(..., now time.Time) (Xxx, error)`
- State transitions as methods that return an error when the transition isn't allowed: `(*Xxx) Disable(now time.Time) error`
- Queries and derived values as methods: `IsExpired(now) bool`, `CanBeUsedBy(id) bool`, `DisplayName() string`
- Policies that span several entities, as plain functions in `domain`
- Domain errors: all of them in `domain/errors.go` (see `architecture.md` → Errors)
- Pure parse/validate functions for value objects (`ParseEmail(s string) (string, error)`), so a usecase can reject bad input **before** expensive work (hashing, external calls). Constructors reuse the same function.

## Constraints

- No I/O, no `context.Context`, no logging, no infra imports. Domain code is pure and deterministic.
- Never call `time.Now()` in domain. Take `now time.Time` as a parameter (the usecase gets it from the `Clock` port).
- Generate IDs in the constructor: `uuid.Must(uuid.NewV7())`. An entity is complete before it is persisted.
- No struct tags (`json`, `db`). Serialization belongs to dto and repository.
- Fields are exported for reading. Mutation goes only through constructors and methods. Other layers must not assign entity fields directly, except the repository when it rebuilds an entity from a DB row (trusted data, no re-validation).

## Example (illustrative only, not a spec)

```go
type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

type Connection struct {
	ID        uuid.UUID
	Name      string
	Status    Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewConnection(name string, now time.Time) (Connection, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Connection{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	return Connection{
		ID:        uuid.Must(uuid.NewV7()),
		Name:      name,
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (c *Connection) Disable(now time.Time) error {
	if c.Status == StatusDisabled {
		return fmt.Errorf("%w: connection is already disabled", ErrInvalidState)
	}
	c.Status = StatusDisabled
	c.UpdatedAt = now
	return nil
}
```

Usecase calls it, and does not re-implement it:

```go
// ❌ the usecase decides
if conn.Status == domain.StatusDisabled {
	return domain.Connection{}, errors.New("already disabled")
}
conn.Status = domain.StatusDisabled

// ✅ the domain decides
if err := conn.Disable(u.clock.Now()); err != nil {
	return domain.Connection{}, err
}
```

## errors.go skeleton

```go
var (
	ErrValidation   = errors.New("validation failed")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalidState = errors.New("invalid state")
	ErrForbidden    = errors.New("forbidden")
)

// user
var (
	ErrUserNotFound = fmt.Errorf("user %w", ErrNotFound)
	ErrEmailTaken   = fmt.Errorf("%w: email already taken", ErrConflict)
)
```
