package create

import (
	"context"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
)

//go:generate go tool mockgen -source $GOFILE -destination mocks_test.go -package create_test

type (
	tripRepository interface {
		Create(ctx context.Context, entity domaintrip.Trip) error
	}

	statusHistoryRepository interface {
		InsertStatus(ctx context.Context, tripID uuid.UUID, fromStatus *domaintrip.Status, toStatus domaintrip.Status, reason string, changedAt time.Time) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(context.Context) error) error
	}

	metrics interface {
		TripCreated(ctx context.Context)
	}

	clock interface {
		Now() time.Time
	}

	idGenerator interface {
		New() uuid.UUID
	}
)
