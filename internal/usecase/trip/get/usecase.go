package get

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
)

type UseCase struct {
	trips tripRepository
}

func New(trips tripRepository) *UseCase {
	return &UseCase{trips: trips}
}

func (uc *UseCase) Execute(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error) {
	entity, err := uc.trips.GetByID(ctx, id)
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("get trip: %w", err)
	}
	return entity, nil
}
