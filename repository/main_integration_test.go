//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

const defaultTestDatabaseURL = "postgres://cm:cm@localhost:5441/connection_manager_test?sslmode=disable"

// testPool is shared by every integration test in this package.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	os.Exit(runIntegration(m))
}

func runIntegration(m *testing.M) int {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = defaultTestDatabaseURL
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration: connect:", err)
		return 1
	}
	defer pool.Close()
	if err := resetSchema(ctx, pool); err != nil {
		fmt.Fprintln(os.Stderr, "integration: migrate:", err)
		return 1
	}
	testPool = pool
	return m.Run()
}

// resetSchema drops everything and applies every *.up.sql in order.
func resetSchema(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join("..", "db", "migrations", "*.up.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no migrations found")
	}
	sort.Strings(files)
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
	}
	return nil
}

// truncate empties the given tables; call at the start of each test.
func truncate(t *testing.T, tables ...string) {
	t.Helper()
	for _, tbl := range tables {
		_, err := testPool.Exec(context.Background(), "TRUNCATE TABLE "+tbl+" CASCADE")
		require.NoError(t, err)
	}
}
