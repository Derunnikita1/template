package save

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
)

type Input struct {
	TripID     uuid.UUID
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
}

type UseCase struct {
	repository positionRepository
	txManager  txManager
	metrics    metrics
	clock      clock
	queue      queue
}

func New(repository positionRepository, txManager txManager, metrics metrics, clock clock, queue queue) *UseCase {
	return &UseCase{repository: repository, txManager: txManager, metrics: metrics, clock: clock, queue: queue}
}

func (u *UseCase) Execute(ctx context.Context, input Input) (domaintrip.Position, error) {
	entity, err := domaintrip.NewTripPosition(domaintrip.NewPosition{
		TripID:     input.TripID,
		Latitude:   input.Latitude,
		Longitude:  input.Longitude,
		RecordedAt: input.RecordedAt,
		Now:        u.clock.Now(),
	})
	if err != nil {
		return domaintrip.Position{}, err
	}

	err = u.txManager.Do(ctx, func(txCtx context.Context) error {
		if err := u.repository.UpdateLastPositionAt(txCtx, entity.TripID, entity.RecordedAt); err != nil {
			return fmt.Errorf("update trip position time: %w", err)
		}
		if err := u.repository.InsertPosition(txCtx, &entity); err != nil {
			return fmt.Errorf("save trip position: %w", err)
		}
		return nil
	})
	if err != nil {
		return domaintrip.Position{}, err
	}

	u.queue.Cancel(entity.TripID)
	u.metrics.PositionSaved(ctx)
	return entity, nil
}
