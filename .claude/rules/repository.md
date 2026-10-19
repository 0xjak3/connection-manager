---
paths:
  - "repository/**"
  - "db/**"
  - "sqlc.yaml"
---

# Repository, SQL and sqlc rules

## sqlc.yaml (baseline)

```yaml
version: "2"
sql:
  - engine: postgresql
    schema: db/migrations
    queries: db/queries
    gen:
      go:
        package: sqlcgen
        out: repository/sqlcgen
        sql_package: pgx/v5
        emit_empty_slices: true
        emit_pointers_for_null_types: true
        overrides:
          - db_type: uuid
            go_type: github.com/google/uuid.UUID
          - db_type: uuid
            nullable: true
            go_type:
              import: github.com/google/uuid
              type: UUID
              pointer: true
          - db_type: timestamptz
            go_type: time.Time
          - db_type: timestamptz
            nullable: true
            go_type:
              import: time
              type: Time
              pointer: true
```

## Migrations (`db/migrations/`)

- golang-migrate naming: `000001_create_connections.up.sql` + `000001_create_connections.down.sql`. Always write both.
- Never edit a migration that has already been applied. Add a new one.
- Tables are snake_case plural. Every table has `id uuid PRIMARY KEY`, `created_at timestamptz NOT NULL`, `updated_at timestamptz NOT NULL`. IDs and timestamps come from the domain, not from DB defaults.
- Enforce invariants in the DB too (NOT NULL, UNIQUE, CHECK, FK) as a safety net. The domain is still the primary guard.

## Queries (`db/queries/<table>.sql`)

- Name queries `-- name: GetConnectionByID :one`, using the verbs `Get`, `List`, `Create`, `Update`, `Delete`, `Upsert`, `Count`.
- Use `sqlc.arg(name)` / `sqlc.narg(name)` so the generated params have readable names.
- Run `sqlc generate` after every change. Never hand-edit `repository/sqlcgen/` (the hook blocks it).

## Repository code (`repository/<aggregate>.go`)

```go
type ConnectionRepository struct{ q *sqlcgen.Queries }

func NewConnectionRepository(db sqlcgen.DBTX) *ConnectionRepository {
	return &ConnectionRepository{q: sqlcgen.New(db)}
}

func (r *ConnectionRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Connection, error) {
	row, err := r.q.GetConnectionByID(ctx, id)
	if err != nil {
		return domain.Connection{}, translateErr("get connection", err, domain.ErrConnectionNotFound, nil)
	}
	return connectionFromRow(row), nil
}
```

- Public methods take and return **domain** types (or primitives). sqlcgen types never cross the package boundary.
- Mapping functions are private and live next to the repository: `connectionFromRow(sqlcgen.Connection) domain.Connection`, `createConnectionParams(domain.Connection) sqlcgen.CreateConnectionParams`.
- No business logic. The repository stores and loads; it does not decide. Filtering belongs in SQL, not in Go loops over rows.
- Translate every error with `translateErr(op string, err error, notFound, conflict error) error` from `repository/errors.go`:
  - `pgx.ErrNoRows` → `notFound` (the entity-specific not-found error)
  - `*pgconn.PgError` code `23505` (unique_violation) → `conflict` (an entity-specific error such as `domain.ErrEmailTaken`), or `fmt.Errorf("%w: <constraint>", domain.ErrConflict)` when `conflict` is nil
  - anything else → `fmt.Errorf("repository: <op>: %w", err)`
  - Pass `nil` for a mapping that can't happen for that query. When a table has several unique constraints, switch on `pgErr.ConstraintName` in the repository method.
- `:exec` updates and deletes return the affected-row count (`:execrows`). 0 rows → the not-found error.
- Times read from rows are normalized with `.UTC()` in the row mapper (see `architecture.md` → Time).

## Transactions

Not built yet. When the first use case needs atomic multi-write, add `repository.TxManager` with `WithinTx(ctx, func(ctx context.Context) error) error`. It carries the `pgx.Tx` in ctx, and repositories use `q.WithTx(tx)` when one is present. The usecase depends on a `TxManager` port. Record the final design here.
