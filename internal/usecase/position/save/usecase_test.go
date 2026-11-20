package save_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	positionsave "github.com/Derunnikita1/template/internal/usecase/position/save"
)

func TestExecuteSavesPositionAndCancelsQueue(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := NewMockpositionRepository(ctrl)
	txManager := NewMocktxManager(ctrl)
	metrics := NewMockmetrics(ctrl)
	clock := NewMockclock(ctrl)
	queue := NewMockqueue(ctrl)
	now := time.Date(2026, time.November, 20, 12, 0, 0, 0, time.UTC)
	tripID := uuid.New()
	clock.EXPECT().Now().Return(now)
	txManager.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	)
	repository.EXPECT().UpdateLastPositionAt(gomock.Any(), tripID, now).Return(nil)
	repository.EXPECT().InsertPosition(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, position *domaintrip.Position) error {
			position.ID = 42
			return nil
		},
	)
	queue.EXPECT().Cancel(tripID)
	metrics.EXPECT().PositionSaved(gomock.Any())

	position, err := positionsave.New(repository, txManager, metrics, clock, queue).Execute(
		context.Background(),
		positionsave.Input{TripID: tripID, Latitude: 59.93, Longitude: 30.33, RecordedAt: now},
	)
	if err != nil {
		t.Fatal(err)
	}
	if position.ID != 42 {
		t.Fatalf("position id = %d", position.ID)
	}
}

func TestExecuteDoesNotCancelQueueAfterRollback(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := NewMockpositionRepository(ctrl)
	txManager := NewMocktxManager(ctrl)
	metrics := NewMockmetrics(ctrl)
	clock := NewMockclock(ctrl)
	queue := NewMockqueue(ctrl)
	now := time.Now().UTC()
	tripID := uuid.New()
	clock.EXPECT().Now().Return(now)
	txManager.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
	)
	repository.EXPECT().UpdateLastPositionAt(gomock.Any(), tripID, now).Return(nil)
	repository.EXPECT().InsertPosition(gomock.Any(), gomock.Any()).Return(errors.New("insert failed"))

	_, err := positionsave.New(repository, txManager, metrics, clock, queue).Execute(
		context.Background(),
		positionsave.Input{TripID: tripID, Latitude: 59.93, Longitude: 30.33, RecordedAt: now},
	)
	if err == nil {
		t.Fatal("expected error")
	}
}
