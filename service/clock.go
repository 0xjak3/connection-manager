package service

import "time"

// SystemClock implements usecase.Clock with the real wall clock, in UTC,
// truncated to microseconds (Postgres timestamptz precision) so an entity
// returned right after Create matches the same entity loaded from the DB.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }
