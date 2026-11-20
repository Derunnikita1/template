package trip_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
)

func TestNewTripPosition(t *testing.T) {
	now := time.Date(2026, time.November, 20, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		input   domaintrip.NewPosition
		wantErr bool
	}{
		{
			name: "valid",
			input: domaintrip.NewPosition{
				TripID: uuid.New(), Latitude: 59.93, Longitude: 30.33,
				RecordedAt: now, Now: now,
			},
		},
		{
			name: "latitude outside range",
			input: domaintrip.NewPosition{
				TripID: uuid.New(), Latitude: 91, Longitude: 30.33,
				RecordedAt: now, Now: now,
			},
			wantErr: true,
		},
		{
			name: "longitude outside range",
			input: domaintrip.NewPosition{
				TripID: uuid.New(), Latitude: 59.93, Longitude: 181,
				RecordedAt: now, Now: now,
			},
			wantErr: true,
		},
		{
			name: "too far in future",
			input: domaintrip.NewPosition{
				TripID: uuid.New(), Latitude: 59.93, Longitude: 30.33,
				RecordedAt: now.Add(time.Minute + time.Nanosecond), Now: now,
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			position, err := domaintrip.NewTripPosition(test.input)
			if test.wantErr {
				var validationError domaintrip.ValidationError
				if !errors.As(err, &validationError) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if position.TripID != test.input.TripID || position.RecordedAt != test.input.RecordedAt {
				t.Fatalf("position = %#v", position)
			}
		})
	}
}
