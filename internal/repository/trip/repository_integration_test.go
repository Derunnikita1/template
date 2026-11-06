//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	"github.com/Derunnikita1/template/internal/infra/postgres/transaction"
	triprepository "github.com/Derunnikita1/template/internal/repository/trip"
)

func TestRepositoryLifecycle(t *testing.T) {
	pool := integrationPool(t)
	repository := triprepository.New(pool)
	entity := integrationTrip()
	t.Cleanup(func() { deleteTrip(t, pool, entity.ID) })

	if err := repository.Create(context.Background(), entity); err != nil {
		t.Fatalf("create: %v", err)
	}
	found, err := repository.GetByID(context.Background(), entity.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if found.ID != entity.ID || found.Status != domaintrip.StatusActive {
		t.Fatalf("unexpected stored trip: %#v", found)
	}

	finishedAt := time.Now().UTC()
	if err := found.Finish(finishedAt); err != nil {
		t.Fatalf("domain finish: %v", err)
	}
	if err := repository.Finish(context.Background(), found); err != nil {
		t.Fatalf("repository finish: %v", err)
	}
	completed, err := repository.GetByID(context.Background(), entity.ID)
	if err != nil {
		t.Fatalf("get completed: %v", err)
	}
	if completed.Status != domaintrip.StatusCompleted {
		t.Fatalf("status = %q, want %q", completed.Status, domaintrip.StatusCompleted)
	}
}

func TestTransactionRollsBackTripAndHistory(t *testing.T) {
	pool := integrationPool(t)
	repository := triprepository.New(pool)
	manager := transaction.NewManager(pool)
	entity := integrationTrip()
	t.Cleanup(func() { deleteTrip(t, pool, entity.ID) })

	err := manager.Do(context.Background(), func(ctx context.Context) error {
		if err := repository.Create(ctx, entity); err != nil {
			return err
		}
		return repository.InsertStatus(ctx, entity.ID, nil, domaintrip.Status("invalid"), "must fail", time.Now())
	})
	if err == nil {
		t.Fatal("expected transaction error")
	}
	_, err = repository.GetByID(context.Background(), entity.ID)
	if !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Fatalf("trip remained after rollback: %v", err)
	}
}

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func integrationTrip() domaintrip.Trip {
	now := time.Now().UTC()
	return domaintrip.Trip{
		ID: uuid.New(), UserID: uuid.New(), DriverID: uuid.New(),
		StartLatitude: 59.9, StartLongitude: 30.3,
		EndLatitude: 59.8, EndLongitude: 30.4,
		Price: 1450, Status: domaintrip.StatusActive,
		StartedAt: now, CreatedAt: now, UpdatedAt: now,
	}
}

func deleteTrip(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "DELETE FROM trips WHERE id = $1", id); err != nil {
		t.Errorf("delete trip: %v", err)
	}
}
