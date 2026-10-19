//go:build e2e

package e2e

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Same image as docker-compose.yml.
const postgresImage = "postgres:17"

// baseURL is the root URL of the API binary under test, e.g. http://127.0.0.1:54321.
var baseURL string

// specRouter matches requests to operations in openapi.yaml; the HTTP helper
// uses it to validate every response against the contract.
var specRouter routers.Router

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return fail("resolve project root", err)
	}

	// 1. Throwaway Postgres container on a random host port.
	pg, err := postgres.Run(ctx, postgresImage,
		postgres.WithDatabase("connection_manager_e2e"),
		postgres.WithUsername("cm"),
		postgres.WithPassword("cm"),
		postgres.BasicWaitStrategies(),
	)
	defer func() {
		if err := testcontainers.TerminateContainer(pg); err != nil {
			fmt.Fprintf(os.Stderr, "e2e: terminate container: %v\n", err)
		}
	}()
	if err != nil {
		return fail("start postgres container", err)
	}
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fail("container connection string", err)
	}

	// 2. Apply db/migrations with golang-migrate (pgx/v5 driver).
	if err := migrateUp(root, dsn); err != nil {
		return fail("apply migrations", err)
	}

	// 3. Build the real binary and start it.
	tmp, err := os.MkdirTemp("", "cm-e2e-*")
	if err != nil {
		return fail("create temp dir", err)
	}
	defer os.RemoveAll(tmp)

	bin := filepath.Join(tmp, "api")
	build := exec.Command("go", "build", "-o", bin, "./cmd/api")
	build.Dir = root
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return fail("build ./cmd/api", err)
	}

	addr, err := freeAddr()
	if err != nil {
		return fail("find free port", err)
	}
	baseURL = "http://" + addr

	if specRouter, err = loadSpec(filepath.Join(root, "api", "openapi.gen.yaml"), baseURL); err != nil {
		return fail("load openapi.yaml", err)
	}

	srv := exec.Command(bin)
	srv.Dir = root
	srv.Env = append(os.Environ(), "DATABASE_URL="+dsn, "HTTP_ADDR="+addr)
	srv.Stdout = os.Stderr
	srv.Stderr = os.Stderr
	if err := srv.Start(); err != nil {
		return fail("start api binary", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- srv.Wait() }()

	// 4. Graceful shutdown with SIGTERM after the tests.
	defer stopServer(srv, exited)

	if err := waitHealthy(baseURL+"/healthz", exited, 30*time.Second); err != nil {
		return fail("wait for /healthz", err)
	}

	return m.Run()
}

// loadSpec loads and validates openapi.yaml and returns a router over it. The
// spec declares no servers, so the router is pointed at the API under test.
func loadSpec(path, serverURL string) (routers.Router, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, err
	}
	if err := doc.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("invalid spec: %w", err)
	}
	doc.Servers = openapi3.Servers{{URL: serverURL}}
	return gorillamux.NewRouter(doc)
}

func migrateUp(root, dsn string) error {
	src := "file://" + filepath.ToSlash(filepath.Join(root, "db", "migrations"))
	// The pgx/v5 driver registers the "pgx5" scheme.
	dbURL := "pgx5" + dsn[len("postgres"):]
	mig, err := migrate.New(src, dbURL)
	if err != nil {
		return err
	}
	defer mig.Close()
	if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func freeAddr() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	return l.Addr().String(), nil
}

func waitHealthy(url string, exited <-chan error, timeout time.Duration) error {
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			return fmt.Errorf("api exited before becoming healthy: %v", err)
		default:
		}
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out after %s", timeout)
}

func stopServer(srv *exec.Cmd, exited <-chan error) {
	if srv.Process == nil {
		return
	}
	_ = srv.Process.Signal(syscall.SIGTERM)
	select {
	case err := <-exited:
		if err != nil {
			fmt.Fprintf(os.Stderr, "e2e: api exited with: %v\n", err)
		}
	case <-time.After(15 * time.Second):
		fmt.Fprintln(os.Stderr, "e2e: api did not stop after SIGTERM, killing it")
		_ = srv.Process.Kill()
		<-exited
	}
}

func fail(step string, err error) int {
	fmt.Fprintf(os.Stderr, "e2e setup: %s: %v\n", step, err)
	return 1
}
