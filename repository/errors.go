package repository

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"connection_manager/domain"
)

const pgUniqueViolation = "23505"

// translateErr maps infrastructure errors to domain errors.
//
//   - pgx.ErrNoRows -> notFound
//   - unique_violation (23505) -> conflict, or a generic domain.ErrConflict when conflict is nil
//   - anything else -> wrapped with the operation name
func translateErr(op string, err error, notFound, conflict error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) && notFound != nil {
		return notFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		if conflict != nil {
			return conflict
		}
		return fmt.Errorf("%w: %s", domain.ErrConflict, pgErr.ConstraintName)
	}
	return fmt.Errorf("repository: %s: %w", op, err)
}
