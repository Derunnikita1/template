package container

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Derunnikita1/template/internal/config"
	"github.com/Derunnikita1/template/internal/http/server"
	createhandler "github.com/Derunnikita1/template/internal/http/trip/create"
	finishhandler "github.com/Derunnikita1/template/internal/http/trip/finish"
	gethandler "github.com/Derunnikita1/template/internal/http/trip/get"
	"github.com/Derunnikita1/template/internal/infra/postgres"
	"github.com/Derunnikita1/template/internal/infra/postgres/transaction"
	"github.com/Derunnikita1/template/internal/infra/telemetry"
	triprepository "github.com/Derunnikita1/template/internal/repository/trip"
	createuc "github.com/Derunnikita1/template/internal/usecase/trip/create"
	finishuc "github.com/Derunnikita1/template/internal/usecase/trip/finish"
	getuc "github.com/Derunnikita1/template/internal/usecase/trip/get"
)

const serviceVersion = "2.0.0"

type App struct {
	server          *http.Server
	pool            *pgxpool.Pool
	telemetry       *telemetry.Provider
	shutdownTimeout time.Duration
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now().UTC()
}

type uuidGenerator struct{}

func (uuidGenerator) New() uuid.UUID {
	return uuid.New()
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	telemetryProvider, err := telemetry.New(ctx, telemetry.Config{
		ServiceName: cfg.OTelServiceName,
		Version:     serviceVersion,
		LogLevel:    cfg.LogLevel,
		Sampler:     cfg.OTelTracesSampler,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize telemetry: %w", err)
	}

	pool, err := postgres.NewPool(ctx, postgres.Config{
		URL:             cfg.DatabaseURL,
		MaxConns:        cfg.DatabaseMaxConns,
		MinConns:        cfg.DatabaseMinConns,
		MaxConnLifetime: cfg.DatabaseMaxConnLifetime,
		ConnectTimeout:  cfg.DatabaseConnectTimeout,
	}, telemetryProvider.Meter.Meter("github.com/Derunnikita1/template/database"))
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		_ = telemetryProvider.Shutdown(shutdownCtx)
		return nil, err
	}

	repository := triprepository.New(pool)
	countCtx, cancel := context.WithTimeout(ctx, cfg.DatabaseQueryTimeout)
	activeTrips, err := repository.CountActive(countCtx)
	cancel()
	if err != nil {
		pool.Close()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer shutdownCancel()
		_ = telemetryProvider.Shutdown(shutdownCtx)
		return nil, err
	}
	telemetryProvider.Metrics.SetActiveTrips(activeTrips)

	txManager := transaction.NewManager(pool)
	clock := systemClock{}
	ids := uuidGenerator{}
	createUseCase := createuc.New(repository, repository, txManager, telemetryProvider.Metrics, clock, ids, telemetryProvider.Logger)
	getUseCase := getuc.New(repository)
	finishUseCase := finishuc.New(repository, repository, txManager, telemetryProvider.Metrics, clock, telemetryProvider.Logger)

	handler := server.NewHandler(
		createhandler.New(createUseCase, telemetryProvider.Logger),
		gethandler.New(getUseCase, telemetryProvider.Logger),
		finishhandler.New(finishUseCase, telemetryProvider.Logger),
		pool,
		server.Config{QueryTimeout: cfg.DatabaseQueryTimeout},
	)

	return &App{
		server: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           server.NewRouter(handler, telemetryProvider.Logger, telemetryProvider.Metrics),
			ReadTimeout:       cfg.HTTPReadTimeout,
			ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
			WriteTimeout:      cfg.HTTPWriteTimeout,
			IdleTimeout:       cfg.HTTPIdleTimeout,
		},
		pool:            pool,
		telemetry:       telemetryProvider,
		shutdownTimeout: cfg.ShutdownTimeout,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	a.telemetry.Logger.Info("service started", "addr", a.server.Addr)
	serverErrors := make(chan error, 1)
	go func() {
		err := a.server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serverErrors <- err
	}()

	var runErr error
	select {
	case <-ctx.Done():
		a.telemetry.Logger.Info("shutdown started")
	case runErr = <-serverErrors:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()
	serverErr := a.server.Shutdown(shutdownCtx)
	a.pool.Close()
	telemetryErr := a.telemetry.Shutdown(shutdownCtx)
	a.telemetry.Logger.Info("service stopped")

	return errors.Join(runErr, serverErr, telemetryErr)
}
