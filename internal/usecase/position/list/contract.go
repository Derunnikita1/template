package list

import (
	"context"

	"github.com/google/uuid"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
)

//go:generate go tool mockgen -source $GOFILE -destination mocks_test.go -package list_test

type repository interface {
	GetByID(ctx context.Context, tripID uuid.UUID) (domaintrip.Trip, error)
	ListPositions(ctx context.Context, tripID uuid.UUID) ([]domaintrip.Position, error)
}
