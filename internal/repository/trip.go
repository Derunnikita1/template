package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Derunnikita1/template/internal/domain"
	"github.com/Derunnikita1/template/internal/tx"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type Trip struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
	Status         string
	StartedAt      time.Time
	FinishedAt     *time.Time
	LastPositionAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type TripRepository struct {
	pool *pgxpool.Pool
}

func NewTripRepository(pool *pgxpool.Pool) *TripRepository {
	return &TripRepository{pool: pool}
}

func (r *TripRepository) Create(ctx context.Context, trip Trip) error {
	q := tx.QuerierFromContext(ctx, r.pool)
	query, args, err := psql.
		Insert("trips").
		Columns(
			"id",
			"user_id",
			"driver_id",
			"start_latitude",
			"start_longitude",
			"end_latitude",
			"end_longitude",
			"price",
			"status",
			"started_at",
			"finished_at",
			"created_at",
			"updated_at",
		).
		Values(
			trip.ID,
			trip.UserID,
			trip.DriverID,
			trip.StartLatitude,
			trip.StartLongitude,
			trip.EndLatitude,
			trip.EndLongitude,
			trip.Price,
			trip.Status,
			trip.StartedAt,
			trip.FinishedAt,
			trip.CreatedAt,
			trip.UpdatedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert trip: %w", err)
	}

	_, err = q.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDriverBusy
		}
		return fmt.Errorf("insert trip: %w", err)
	}
	return nil
}

func (r *TripRepository) GetByID(ctx context.Context, id uuid.UUID) (Trip, error) {
	q := tx.QuerierFromContext(ctx, r.pool)
	query, args, err := psql.
		Select(
			"id",
			"user_id",
			"driver_id",
			"start_latitude",
			"start_longitude",
			"end_latitude",
			"end_longitude",
			"price",
			"status",
			"started_at",
			"finished_at",
			"created_at",
			"updated_at",
		).
		From("trips").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return Trip{}, fmt.Errorf("build get trip: %w", err)
	}

	var trip Trip
	err = q.QueryRow(ctx, query, args...).Scan(
		&trip.ID,
		&trip.UserID,
		&trip.DriverID,
		&trip.StartLatitude,
		&trip.StartLongitude,
		&trip.EndLatitude,
		&trip.EndLongitude,
		&trip.Price,
		&trip.Status,
		&trip.StartedAt,
		&trip.FinishedAt,
		&trip.CreatedAt,
		&trip.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Trip{}, domain.ErrTripNotFound
	}
	if err != nil {
		return Trip{}, fmt.Errorf("get trip: %w", err)
	}
	return trip, nil
}

func (r *TripRepository) Finish(ctx context.Context, id uuid.UUID, finishedAt time.Time) (Trip, error) {
	q := tx.QuerierFromContext(ctx, r.pool)
	query, args, err := psql.
		Update("trips").
		Set("status", "completed").
		Set("finished_at", finishedAt).
		Set("updated_at", finishedAt).
		Where(sq.Eq{"id": id, "status": "active"}).
		Suffix("RETURNING id, user_id, driver_id, start_latitude, start_longitude, end_latitude, end_longitude, price, status, started_at, finished_at, created_at, updated_at").
		ToSql()
	if err != nil {
		return Trip{}, fmt.Errorf("build finish trip: %w", err)
	}

	var trip Trip
	err = q.QueryRow(ctx, query, args...).Scan(
		&trip.ID,
		&trip.UserID,
		&trip.DriverID,
		&trip.StartLatitude,
		&trip.StartLongitude,
		&trip.EndLatitude,
		&trip.EndLongitude,
		&trip.Price,
		&trip.Status,
		&trip.StartedAt,
		&trip.FinishedAt,
		&trip.CreatedAt,
		&trip.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, getErr := r.GetByID(ctx, id)
		if getErr != nil {
			return Trip{}, getErr
		}
		if existing.Status == "completed" {
			return Trip{}, domain.ErrTripCompleted
		}
		return Trip{}, domain.ErrTripNotFound
	}
	if err != nil {
		return Trip{}, fmt.Errorf("finish trip: %w", err)
	}
	return trip, nil
}

type StatusHistoryRepository struct {
	pool *pgxpool.Pool
}

func NewStatusHistoryRepository(pool *pgxpool.Pool) *StatusHistoryRepository {
	return &StatusHistoryRepository{pool: pool}
}

func (r *StatusHistoryRepository) Insert(ctx context.Context, tripID uuid.UUID, fromStatus *string, toStatus string, reason *string, changedAt time.Time) error {
	q := tx.QuerierFromContext(ctx, r.pool)
	query, args, err := psql.
		Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason", "changed_at").
		Values(tripID, fromStatus, toStatus, reason, changedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert status history: %w", err)
	}

	_, err = q.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("insert status history: %w", err)
	}
	return nil
}
