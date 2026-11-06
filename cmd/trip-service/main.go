package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/Derunnikita1/template/internal/config"
	"github.com/Derunnikita1/template/internal/container"
)

func main() {
	os.Exit(run())
}

func run() int {
	_ = godotenv.Load()
	fallback := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "trip-service")

	cfg, err := config.Load()
	if err != nil {
		fallback.Error("load configuration", "err", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := container.New(ctx, cfg)
	if err != nil {
		fallback.Error("initialize service", "err", err)
		return 1
	}
	if err := app.Run(ctx); err != nil {
		fallback.Error("service stopped with error", "err", err)
		return 1
	}
	return 0
}
