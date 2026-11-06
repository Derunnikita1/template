package create

import (
	"context"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	createuc "github.com/Derunnikita1/template/internal/usecase/trip/create"
)

//go:generate go tool mockgen -source $GOFILE -destination mocks_test.go -package create_test

type useCase interface {
	Execute(ctx context.Context, input createuc.Input) (domaintrip.Trip, error)
}
