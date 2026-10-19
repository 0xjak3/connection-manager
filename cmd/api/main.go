package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"connection_manager/handler"
	"connection_manager/repository"
	"connection_manager/router"
	"connection_manager/service"
	"connection_manager/usecase"
	"connection_manager/util"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := util.LoadConfig()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}

	userRepo := repository.NewUserRepository(pool)
	passwords := service.NewPasswordService(service.PasswordConfig{})
	clock := service.SystemClock{}
	userUC := usecase.NewUserUsecase(userRepo, passwords, clock)

	app := router.New(router.Handlers{
		User: handler.NewUserHandler(userUC),
	})

	errCh := make(chan error, 1)
	go func() { errCh <- app.Listen(cfg.HTTPAddr) }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return app.ShutdownWithContext(shutdownCtx)
}
