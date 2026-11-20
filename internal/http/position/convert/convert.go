package convert

import (
	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	api "github.com/Derunnikita1/template/internal/generated"
)

func ToAPI(entity domaintrip.Position) api.TripPosition {
	return api.TripPosition{
		Id:          entity.ID,
		TripId:      entity.TripID,
		Coordinates: api.Coordinates{Latitude: entity.Latitude, Longitude: entity.Longitude},
		RecordedAt:  entity.RecordedAt,
	}
}

func ListToAPI(entities []domaintrip.Position) []api.TripPosition {
	positions := make([]api.TripPosition, 0, len(entities))
	for _, entity := range entities {
		positions = append(positions, ToAPI(entity))
	}
	return positions
}
