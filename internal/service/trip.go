package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Derunnikita1/template/internal/repository"
	"github.com/Derunnikita1/template/internal/tx"
)

type TripService struct {
	txManager *tx.Manager
	trips     *repository.TripRepository
	history   *repository.StatusHistoryRepository
	now       func() time.Time
}

func NewTripService(
	txManager *tx.Manager,
	trips *repository.TripRepository,
	history *repository.StatusHistoryRepository,
) *TripService {
	return &TripService{
		txManager: txManager,
		trips:     trips,
		history:   history,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

type CreateTripInput struct {
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
}

func (s *TripService) Create(ctx context.Context, in CreateTripInput) (repository.Trip, error) {
	now := s.now()
	trip := repository.Trip{
		ID:             uuid.New(),
		UserID:         in.UserID,
		DriverID:       in.DriverID,
		StartLatitude:  in.StartLatitude,
		StartLongitude: in.StartLongitude,
		EndLatitude:    in.EndLatitude,
		EndLongitude:   in.EndLongitude,
		Price:          in.Price,
		Status:         "active",
		StartedAt:      now,
		FinishedAt:     nil,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		if err := s.trips.Create(ctx, trip); err != nil {
			return err
		}
		reason := "trip created"
		if err := s.history.Insert(ctx, trip.ID, nil, "active", &reason, now); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return repository.Trip{}, err
	}
	return trip, nil
}

func (s *TripService) Get(ctx context.Context, id uuid.UUID) (repository.Trip, error) {
	return s.trips.GetByID(ctx, id)
}

func (s *TripService) Finish(ctx context.Context, id uuid.UUID) (repository.Trip, error) {
	var trip repository.Trip
	now := s.now()
	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		finished, err := s.trips.Finish(ctx, id, now)
		if err != nil {
			return err
		}
		from := "active"
		reason := "trip finished"
		if err := s.history.Insert(ctx, finished.ID, &from, "completed", &reason, now); err != nil {
			return err
		}
		trip = finished
		return nil
	})
	if err != nil {
		return repository.Trip{}, err
	}
	return trip, nil
}
