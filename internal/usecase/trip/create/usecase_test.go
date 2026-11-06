package create_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	createuc "github.com/Derunnikita1/template/internal/usecase/trip/create"
)

func TestUseCaseExecuteSuccess(t *testing.T) {
	controller := gomock.NewController(t)
	repository := NewMocktripRepository(controller)
	history := NewMockstatusHistoryRepository(controller)
	txManager := NewMocktxManager(controller)
	metrics := NewMockmetrics(controller)
	clock := NewMockclock(controller)
	ids := NewMockidGenerator(controller)
	now := time.Date(2026, time.November, 6, 12, 0, 0, 0, time.UTC)
	id := uuid.New()
	userID := uuid.New()
	driverID := uuid.New()
	want := domaintrip.Trip{
		ID: id, UserID: userID, DriverID: driverID,
		StartLatitude: 59.9, StartLongitude: 30.3,
		EndLatitude: 59.8, EndLongitude: 30.4,
		Price: 1450, Status: domaintrip.StatusActive,
		StartedAt: now, CreatedAt: now, UpdatedAt: now,
	}

	clock.EXPECT().Now().Return(now)
	ids.EXPECT().New().Return(id)
	txManager.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	)
	repository.EXPECT().Create(gomock.Any(), want).Return(nil)
	history.EXPECT().InsertStatus(gomock.Any(), id, nil, domaintrip.StatusActive, "trip created", now).Return(nil)
	metrics.EXPECT().TripCreated(gomock.Any())

	useCase := createuc.New(repository, history, txManager, metrics, clock, ids, testLogger())
	got, err := useCase.Execute(context.Background(), createuc.Input{
		UserID: userID, DriverID: driverID,
		StartLatitude: 59.9, StartLongitude: 30.3,
		EndLatitude: 59.8, EndLongitude: 30.4, Price: 1450,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got != want {
		t.Fatalf("trip = %#v, want %#v", got, want)
	}
}

func TestUseCaseExecuteDriverBusy(t *testing.T) {
	controller := gomock.NewController(t)
	repository := NewMocktripRepository(controller)
	history := NewMockstatusHistoryRepository(controller)
	txManager := NewMocktxManager(controller)
	metrics := NewMockmetrics(controller)
	clock := NewMockclock(controller)
	ids := NewMockidGenerator(controller)

	clock.EXPECT().Now().Return(time.Now())
	ids.EXPECT().New().Return(uuid.New())
	txManager.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	)
	repository.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domaintrip.ErrDriverBusy)

	useCase := createuc.New(repository, history, txManager, metrics, clock, ids, testLogger())
	_, err := useCase.Execute(context.Background(), createuc.Input{
		UserID: uuid.New(), DriverID: uuid.New(), Price: 10,
	})
	if err == nil || !containsError(err, domaintrip.ErrDriverBusy) {
		t.Fatalf("error = %v, want driver busy", err)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
