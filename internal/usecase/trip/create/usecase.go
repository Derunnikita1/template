package create

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
)

type Input struct {
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
}

type UseCase struct {
	trips   tripRepository
	history statusHistoryRepository
	tx      txManager
	metrics metrics
	clock   clock
	ids     idGenerator
	log     *slog.Logger
}

func New(
	trips tripRepository,
	history statusHistoryRepository,
	tx txManager,
	metrics metrics,
	clock clock,
	ids idGenerator,
	log *slog.Logger,
) *UseCase {
	return &UseCase{
		trips: trips, history: history, tx: tx, metrics: metrics,
		clock: clock, ids: ids, log: log,
	}
}

func (uc *UseCase) Execute(ctx context.Context, input Input) (domaintrip.Trip, error) {
	now := uc.clock.Now()
	entity, err := domaintrip.New(domaintrip.NewTrip{
		ID:             uc.ids.New(),
		UserID:         input.UserID,
		DriverID:       input.DriverID,
		StartLatitude:  input.StartLatitude,
		StartLongitude: input.StartLongitude,
		EndLatitude:    input.EndLatitude,
		EndLongitude:   input.EndLongitude,
		Price:          input.Price,
		Now:            now,
	})
	if err != nil {
		return domaintrip.Trip{}, err
	}

	err = uc.tx.Do(ctx, func(txCtx context.Context) error {
		if createErr := uc.trips.Create(txCtx, entity); createErr != nil {
			return fmt.Errorf("create trip: %w", createErr)
		}
		if historyErr := uc.history.InsertStatus(txCtx, entity.ID, nil, domaintrip.StatusActive, "trip created", now); historyErr != nil {
			return fmt.Errorf("insert trip status: %w", historyErr)
		}
		return nil
	})
	if err != nil {
		return domaintrip.Trip{}, err
	}

	uc.metrics.TripCreated(ctx)
	uc.log.InfoContext(ctx, "trip created", "trip_id", entity.ID.String())

	return entity, nil
}
