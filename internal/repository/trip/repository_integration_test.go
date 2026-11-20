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

func TestListStaleTrips(t *testing.T) {
	pool := integrationPool(t)
	repository := triprepository.New(pool)
	now := time.Now().UTC()
	stale := integrationTrip()
	stale.StartedAt = now.Add(-2 * time.Hour)
	stale.CreatedAt = stale.StartedAt
	stale.UpdatedAt = stale.StartedAt
	fresh := integrationTrip()
	fresh.StartedAt = now.Add(-2 * time.Hour)
	fresh.CreatedAt = fresh.StartedAt
	fresh.UpdatedAt = fresh.StartedAt
	fresh.LastPositionAt = &now
	completed := integrationTrip()
	completed.Status = domaintrip.StatusCompleted
	completed.StartedAt = now.Add(-2 * time.Hour)
	completed.CreatedAt = completed.StartedAt
	completed.UpdatedAt = now
	completed.FinishedAt = &now
	for _, entity := range []domaintrip.Trip{stale, fresh, completed} {
		if err := repository.Create(context.Background(), entity); err != nil {
			t.Fatalf("create trip: %v", err)
		}
		t.Cleanup(func() { deleteTrip(t, pool, entity.ID) })
	}

	trips, err := repository.ListStaleTrips(context.Background(), time.Minute)
	if err != nil {
		t.Fatalf("list stale trips: %v", err)
	}
	found := make(map[uuid.UUID]bool, len(trips))
	for _, entity := range trips {
		found[entity.TripID] = true
	}
	if !found[stale.ID] {
		t.Fatal("trip without positions was not selected")
	}
	if found[fresh.ID] {
		t.Fatal("fresh trip was selected")
	}
	if found[completed.ID] {
		t.Fatal("completed trip was selected")
	}
}

func TestSavePositionIsAtomicAndKeepsNewestTimestamp(t *testing.T) {
	pool := integrationPool(t)
	repository := triprepository.New(pool)
	manager := transaction.NewManager(pool)
	entity := integrationTrip()
	entity.StartedAt = time.Now().UTC().Add(-time.Hour)
	entity.CreatedAt = entity.StartedAt
	entity.UpdatedAt = entity.StartedAt
	if err := repository.Create(context.Background(), entity); err != nil {
		t.Fatalf("create trip: %v", err)
	}
	t.Cleanup(func() { deleteTrip(t, pool, entity.ID) })

	freshTime := time.Now().UTC().Truncate(time.Microsecond)
	freshPosition := domaintrip.Position{
		TripID: entity.ID, Latitude: 59.93, Longitude: 30.33,
		RecordedAt: freshTime, CreatedAt: freshTime,
	}
	if err := manager.Do(context.Background(), func(ctx context.Context) error {
		if err := repository.UpdateLastPositionAt(ctx, entity.ID, freshTime); err != nil {
			return err
		}
		return repository.InsertPosition(ctx, &freshPosition)
	}); err != nil {
		t.Fatalf("save fresh position: %v", err)
	}

	olderTime := freshTime.Add(-time.Hour)
	olderPosition := domaintrip.Position{
		TripID: entity.ID, Latitude: 59.92, Longitude: 30.32,
		RecordedAt: olderTime, CreatedAt: freshTime,
	}
	if err := manager.Do(context.Background(), func(ctx context.Context) error {
		if err := repository.UpdateLastPositionAt(ctx, entity.ID, olderTime); err != nil {
			return err
		}
		return repository.InsertPosition(ctx, &olderPosition)
	}); err != nil {
		t.Fatalf("save older position: %v", err)
	}

	stored, err := repository.GetByID(context.Background(), entity.ID)
	if err != nil {
		t.Fatalf("get trip: %v", err)
	}
	if stored.LastPositionAt == nil || !stored.LastPositionAt.Equal(freshTime) {
		t.Fatalf("last_position_at = %v, want %v", stored.LastPositionAt, freshTime)
	}
	positions, err := repository.ListPositions(context.Background(), entity.ID)
	if err != nil {
		t.Fatalf("list positions: %v", err)
	}
	if len(positions) != 2 || positions[0].ID != olderPosition.ID || positions[1].ID != freshPosition.ID {
		t.Fatalf("positions are not sorted: %#v", positions)
	}

	failedTime := freshTime.Add(time.Minute)
	failedPosition := domaintrip.Position{
		TripID: entity.ID, Latitude: 999, Longitude: 30.31,
		RecordedAt: failedTime, CreatedAt: failedTime,
	}
	err = manager.Do(context.Background(), func(ctx context.Context) error {
		if err := repository.UpdateLastPositionAt(ctx, entity.ID, failedTime); err != nil {
			return err
		}
		return repository.InsertPosition(ctx, &failedPosition)
	})
	if err == nil {
		t.Fatal("expected insert error")
	}
	stored, err = repository.GetByID(context.Background(), entity.ID)
	if err != nil {
		t.Fatalf("get trip after rollback: %v", err)
	}
	if stored.LastPositionAt == nil || !stored.LastPositionAt.Equal(freshTime) {
		t.Fatalf("last_position_at changed after rollback: %v", stored.LastPositionAt)
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
