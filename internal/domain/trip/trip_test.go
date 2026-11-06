package trip_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Derunnikita1/template/internal/domain/trip"
)

func TestNew(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.November, 6, 12, 0, 0, 0, time.UTC)
	valid := trip.NewTrip{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  59.9398,
		StartLongitude: 30.3146,
		EndLatitude:    59.929,
		EndLongitude:   30.3626,
		Price:          1450,
		Now:            now,
	}

	tests := []struct {
		name    string
		change  func(*trip.NewTrip)
		wantErr bool
	}{
		{name: "success"},
		{name: "empty user", change: func(input *trip.NewTrip) { input.UserID = uuid.Nil }, wantErr: true},
		{name: "empty driver", change: func(input *trip.NewTrip) { input.DriverID = uuid.Nil }, wantErr: true},
		{name: "invalid latitude", change: func(input *trip.NewTrip) { input.StartLatitude = 91 }, wantErr: true},
		{name: "invalid longitude", change: func(input *trip.NewTrip) { input.EndLongitude = -181 }, wantErr: true},
		{name: "negative price", change: func(input *trip.NewTrip) { input.Price = -1 }, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input := valid
			if test.change != nil {
				test.change(&input)
			}

			got, err := trip.New(input)
			if test.wantErr {
				var validationError trip.ValidationError
				if !errors.As(err, &validationError) {
					t.Fatalf("expected validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("new trip: %v", err)
			}
			if got.Status != trip.StatusActive {
				t.Fatalf("status = %q, want %q", got.Status, trip.StatusActive)
			}
			if got.StartedAt != now || got.CreatedAt != now || got.UpdatedAt != now {
				t.Fatal("timestamps were not initialized from input time")
			}
		})
	}
}

func TestTripFinish(t *testing.T) {
	t.Parallel()

	finishedAt := time.Date(2026, time.November, 6, 13, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		status  trip.Status
		wantErr error
	}{
		{name: "active", status: trip.StatusActive},
		{name: "completed", status: trip.StatusCompleted, wantErr: trip.ErrTripCompleted},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			entity := trip.Trip{Status: test.status}
			err := entity.Finish(finishedAt)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("finish error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr != nil {
				return
			}
			if entity.Status != trip.StatusCompleted {
				t.Fatalf("status = %q, want %q", entity.Status, trip.StatusCompleted)
			}
			if entity.FinishedAt == nil || !entity.FinishedAt.Equal(finishedAt) {
				t.Fatalf("finished_at = %v, want %v", entity.FinishedAt, finishedAt)
			}
		})
	}
}
