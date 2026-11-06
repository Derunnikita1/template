package finish_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	finishuc "github.com/Derunnikita1/template/internal/usecase/trip/finish"
)

func TestUseCaseExecuteSuccess(t *testing.T) {
	controller := gomock.NewController(t)
	repository := NewMocktripRepository(controller)
	history := NewMockstatusHistoryRepository(controller)
	txManager := NewMocktxManager(controller)
	metrics := NewMockmetrics(controller)
	clock := NewMockclock(controller)
	id := uuid.New()
	now := time.Date(2026, time.November, 6, 13, 0, 0, 0, time.UTC)
	active := domaintrip.Trip{ID: id, Status: domaintrip.StatusActive}
	completed := active
	completed.Status = domaintrip.StatusCompleted
	completed.FinishedAt = &now
	completed.UpdatedAt = now

	clock.EXPECT().Now().Return(now)
	txManager.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	)
	repository.EXPECT().GetByID(gomock.Any(), id).Return(active, nil)
	repository.EXPECT().Finish(gomock.Any(), completed).Return(nil)
	history.EXPECT().InsertStatus(
		gomock.Any(), id, pointer(domaintrip.StatusActive), domaintrip.StatusCompleted, "trip finished", now,
	).Return(nil)
	metrics.EXPECT().TripCompleted(gomock.Any())

	useCase := finishuc.New(repository, history, txManager, metrics, clock, finishLogger())
	got, err := useCase.Execute(context.Background(), id)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got.Status != domaintrip.StatusCompleted || got.FinishedAt == nil || !got.FinishedAt.Equal(now) {
		t.Fatalf("unexpected completed trip: %#v", got)
	}
}

func TestUseCaseExecuteCompleted(t *testing.T) {
	controller := gomock.NewController(t)
	repository := NewMocktripRepository(controller)
	history := NewMockstatusHistoryRepository(controller)
	txManager := NewMocktxManager(controller)
	metrics := NewMockmetrics(controller)
	clock := NewMockclock(controller)
	id := uuid.New()
	now := time.Now()

	clock.EXPECT().Now().Return(now)
	txManager.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	)
	repository.EXPECT().GetByID(gomock.Any(), id).Return(
		domaintrip.Trip{ID: id, Status: domaintrip.StatusCompleted}, nil,
	)

	useCase := finishuc.New(repository, history, txManager, metrics, clock, finishLogger())
	_, err := useCase.Execute(context.Background(), id)
	if !errors.Is(err, domaintrip.ErrTripCompleted) {
		t.Fatalf("error = %v, want trip completed", err)
	}
}

func pointer(status domaintrip.Status) *domaintrip.Status {
	return &status
}

func finishLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
