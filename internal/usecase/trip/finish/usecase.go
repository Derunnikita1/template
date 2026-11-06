package finish

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
)

type UseCase struct {
	trips   tripRepository
	history statusHistoryRepository
	tx      txManager
	metrics metrics
	clock   clock
	log     *slog.Logger
}

func New(
	trips tripRepository,
	history statusHistoryRepository,
	tx txManager,
	metrics metrics,
	clock clock,
	log *slog.Logger,
) *UseCase {
	return &UseCase{trips: trips, history: history, tx: tx, metrics: metrics, clock: clock, log: log}
}

func (uc *UseCase) Execute(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error) {
	var entity domaintrip.Trip
	now := uc.clock.Now()

	err := uc.tx.Do(ctx, func(txCtx context.Context) error {
		found, getErr := uc.trips.GetByID(txCtx, id)
		if getErr != nil {
			return fmt.Errorf("get trip: %w", getErr)
		}
		if finishErr := found.Finish(now); finishErr != nil {
			return finishErr
		}
		if saveErr := uc.trips.Finish(txCtx, found); saveErr != nil {
			return fmt.Errorf("finish trip: %w", saveErr)
		}
		fromStatus := domaintrip.StatusActive
		if historyErr := uc.history.InsertStatus(txCtx, found.ID, &fromStatus, domaintrip.StatusCompleted, "trip finished", now); historyErr != nil {
			return fmt.Errorf("insert trip status: %w", historyErr)
		}
		entity = found
		return nil
	})
	if err != nil {
		return domaintrip.Trip{}, err
	}

	uc.metrics.TripCompleted(ctx)
	uc.log.InfoContext(ctx, "trip completed", "trip_id", entity.ID.String())

	return entity, nil
}
