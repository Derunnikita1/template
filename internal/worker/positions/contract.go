package positions

import (
	"context"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
)

//go:generate go tool mockgen -source $GOFILE -destination mocks_test.go -package positions

type (
	repository interface {
		ListStaleTrips(ctx context.Context, staleAfter time.Duration) ([]domaintrip.StaleTrip, error)
	}

	pushGateway interface {
		RequestPosition(ctx context.Context, tripID, driverID uuid.UUID) error
	}

	metrics interface {
		StaleCheck(ctx context.Context, result string)
		PositionRequestDropped(ctx context.Context)
		PositionRequest(ctx context.Context, result string, duration time.Duration)
		SetPositionQueueSize(size int64)
		PositionWorkerStarted(ctx context.Context)
		PositionWorkerFinished(ctx context.Context)
	}

	ticker interface {
		C() <-chan time.Time
		Stop()
	}

	tickerFactory interface {
		NewTicker(interval time.Duration) ticker
	}
)
