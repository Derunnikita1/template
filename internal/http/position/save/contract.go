package save

import (
	"context"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	positionsave "github.com/Derunnikita1/template/internal/usecase/position/save"
)

//go:generate go tool mockgen -source $GOFILE -destination mocks_test.go -package save_test

type useCase interface {
	Execute(ctx context.Context, input positionsave.Input) (domaintrip.Position, error)
}
