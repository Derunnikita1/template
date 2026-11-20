package save

import (
	"context"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
)

//go:generate go tool mockgen -source $GOFILE -destination mocks_test.go -package save_test

type (
	positionRepository interface {
		UpdateLastPositionAt(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error
		InsertPosition(ctx context.Context, entity *domaintrip.Position) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(context.Context) error) error
	}

	metrics interface {
		PositionSaved(ctx context.Context)
	}

	clock interface {
		Now() time.Time
	}

	queue interface {
		Cancel(tripID uuid.UUID)
	}
)
