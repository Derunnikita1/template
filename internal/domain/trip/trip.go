package trip

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

type Trip struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
	Status         Status
	StartedAt      time.Time
	FinishedAt     *time.Time
	LastPositionAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type NewTrip struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
	Now            time.Time
}

func New(input NewTrip) (Trip, error) {
	if input.UserID == uuid.Nil {
		return Trip{}, ValidationError{Field: "user_id", Reason: "must not be empty"}
	}
	if input.DriverID == uuid.Nil {
		return Trip{}, ValidationError{Field: "driver_id", Reason: "must not be empty"}
	}
	if err := validateCoordinate("start_point.latitude", input.StartLatitude, -90, 90); err != nil {
		return Trip{}, err
	}
	if err := validateCoordinate("start_point.longitude", input.StartLongitude, -180, 180); err != nil {
		return Trip{}, err
	}
	if err := validateCoordinate("end_point.latitude", input.EndLatitude, -90, 90); err != nil {
		return Trip{}, err
	}
	if err := validateCoordinate("end_point.longitude", input.EndLongitude, -180, 180); err != nil {
		return Trip{}, err
	}
	if input.Price < 0 {
		return Trip{}, ValidationError{Field: "price", Reason: "must not be negative"}
	}

	return Trip{
		ID:             input.ID,
		UserID:         input.UserID,
		DriverID:       input.DriverID,
		StartLatitude:  input.StartLatitude,
		StartLongitude: input.StartLongitude,
		EndLatitude:    input.EndLatitude,
		EndLongitude:   input.EndLongitude,
		Price:          input.Price,
		Status:         StatusActive,
		StartedAt:      input.Now,
		CreatedAt:      input.Now,
		UpdatedAt:      input.Now,
	}, nil
}

func (t *Trip) Finish(at time.Time) error {
	if t.Status != StatusActive {
		return ErrTripCompleted
	}

	t.Status = StatusCompleted
	t.FinishedAt = &at
	t.UpdatedAt = at

	return nil
}

func validateCoordinate(field string, value, minValue, maxValue float64) error {
	if value < minValue || value > maxValue {
		return ValidationError{Field: field, Reason: "is outside the allowed range"}
	}

	return nil
}
