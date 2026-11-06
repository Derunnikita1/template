package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
)

//go:generate go tool mockgen -source $GOFILE -destination mocks_test.go -package finish_test

type (
	tripRepository interface {
		GetByID(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error)
		Finish(ctx context.Context, entity domaintrip.Trip) error
	}

	statusHistoryRepository interface {
		InsertStatus(ctx context.Context, tripID uuid.UUID, fromStatus *domaintrip.Status, toStatus domaintrip.Status, reason string, changedAt time.Time) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(context.Context) error) error
	}

	metrics interface {
		TripCompleted(ctx context.Context)
	}

	clock interface {
		Now() time.Time
	}
)
