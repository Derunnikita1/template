package list

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
)

type UseCase struct {
	repository repository
}

func New(repository repository) *UseCase {
	return &UseCase{repository: repository}
}

func (u *UseCase) Execute(ctx context.Context, tripID uuid.UUID) ([]domaintrip.Position, error) {
	if _, err := u.repository.GetByID(ctx, tripID); err != nil {
		return nil, fmt.Errorf("get trip: %w", err)
	}
	positions, err := u.repository.ListPositions(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("list trip positions: %w", err)
	}
	return positions, nil
}
