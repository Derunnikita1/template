package get_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	getuc "github.com/Derunnikita1/template/internal/usecase/trip/get"
)

func TestUseCaseExecute(t *testing.T) {
	controller := gomock.NewController(t)
	repository := NewMocktripRepository(controller)
	id := uuid.New()
	repository.EXPECT().GetByID(gomock.Any(), id).Return(domaintrip.Trip{}, domaintrip.ErrTripNotFound)

	_, err := getuc.New(repository).Execute(context.Background(), id)
	if !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Fatalf("error = %v, want trip not found", err)
	}
}
