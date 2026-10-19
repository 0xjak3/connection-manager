package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSystemClock_Now(t *testing.T) {
	now := SystemClock{}.Now()
	require.Equal(t, time.UTC, now.Location())
	require.Zero(t, now.Nanosecond()%int(time.Microsecond), "must be truncated to microseconds")
}
