package get

import (
	"context"

	"github.com/google/uuid"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
)

//go:generate go tool mockgen -source $GOFILE -destination mocks_test.go -package get_test

type useCase interface {
	Execute(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error)
}
