package list_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	positionlist "github.com/Derunnikita1/template/internal/usecase/position/list"
)

func TestExecuteReturnsRoute(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := NewMockrepository(ctrl)
	tripID := uuid.New()
	want := []domaintrip.Position{{ID: 1, TripID: tripID, RecordedAt: time.Now()}}
	repository.EXPECT().GetByID(gomock.Any(), tripID).Return(domaintrip.Trip{ID: tripID}, nil)
	repository.EXPECT().ListPositions(gomock.Any(), tripID).Return(want, nil)

	got, err := positionlist.New(repository).Execute(context.Background(), tripID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != want[0].ID {
		t.Fatalf("positions = %#v", got)
	}
}

func TestExecuteReturnsTripNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := NewMockrepository(ctrl)
	tripID := uuid.New()
	repository.EXPECT().GetByID(gomock.Any(), tripID).Return(domaintrip.Trip{}, domaintrip.ErrTripNotFound)

	_, err := positionlist.New(repository).Execute(context.Background(), tripID)
	if err == nil {
		t.Fatal("expected error")
	}
}
