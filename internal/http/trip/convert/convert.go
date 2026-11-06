package convert

import (
	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	api "github.com/Derunnikita1/template/internal/generated"
)

func ToAPI(entity domaintrip.Trip) api.Trip {
	return api.Trip{
		Id:       entity.ID,
		UserId:   entity.UserID,
		DriverId: entity.DriverID,
		StartPoint: api.Coordinates{
			Latitude: entity.StartLatitude, Longitude: entity.StartLongitude,
		},
		EndPoint: api.Coordinates{
			Latitude: entity.EndLatitude, Longitude: entity.EndLongitude,
		},
		Price:          entity.Price,
		Status:         api.TripStatus(entity.Status),
		StartedAt:      entity.StartedAt,
		FinishedAt:     entity.FinishedAt,
		LastPositionAt: entity.LastPositionAt,
	}
}
