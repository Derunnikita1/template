package trip

import (
	"time"

	"github.com/google/uuid"
)

const maximumPositionClockSkew = time.Minute

type Position struct {
	ID         int64
	TripID     uuid.UUID
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
	CreatedAt  time.Time
}

type NewPosition struct {
	TripID     uuid.UUID
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
	Now        time.Time
}

func NewTripPosition(input NewPosition) (Position, error) {
	if input.TripID == uuid.Nil {
		return Position{}, ValidationError{Field: "trip_id", Reason: requiredValueReason}
	}
	if err := validateCoordinate("coordinates.latitude", input.Latitude, -90, 90); err != nil {
		return Position{}, err
	}
	if err := validateCoordinate("coordinates.longitude", input.Longitude, -180, 180); err != nil {
		return Position{}, err
	}
	if input.RecordedAt.IsZero() {
		return Position{}, ValidationError{Field: "recorded_at", Reason: "is required"}
	}
	if input.RecordedAt.After(input.Now.Add(maximumPositionClockSkew)) {
		return Position{}, ValidationError{Field: "recorded_at", Reason: "must not be more than one minute in the future"}
	}
	return Position{
		TripID:     input.TripID,
		Latitude:   input.Latitude,
		Longitude:  input.Longitude,
		RecordedAt: input.RecordedAt,
		CreatedAt:  input.Now,
	}, nil
}
