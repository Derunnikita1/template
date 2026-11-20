package trip

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

	domaintrip "github.com/Derunnikita1/template/internal/domain/trip"
	"github.com/Derunnikita1/template/internal/infra/postgres/transaction"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

const statusColumn = "status"

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, entity domaintrip.Trip) error {
	query, args, err := psql.
		Insert("trips").
		Columns(
			"id", "user_id", "driver_id", "start_latitude", "start_longitude",
			"end_latitude", "end_longitude", "price", "status", "started_at",
			"finished_at", "last_position_at", "created_at", "updated_at",
		).
		Values(
			entity.ID, entity.UserID, entity.DriverID, entity.StartLatitude, entity.StartLongitude,
			entity.EndLatitude, entity.EndLongitude, entity.Price, entity.Status, entity.StartedAt,
			entity.FinishedAt, entity.LastPositionAt, entity.CreatedAt, entity.UpdatedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert trip: %w", err)
	}

	querier := transaction.QuerierFromContext(ctx, r.pool)
	if _, err := querier.Exec(ctx, query, args...); err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" {
			return domaintrip.ErrDriverBusy
		}
		return fmt.Errorf("insert trip: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error) {
	query, args, err := psql.
		Select(
			"id", "user_id", "driver_id", "start_latitude", "start_longitude",
			"end_latitude", "end_longitude", "price", "status", "started_at",
			"finished_at", "last_position_at", "created_at", "updated_at",
		).
		From("trips").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("build get trip: %w", err)
	}

	querier := transaction.QuerierFromContext(ctx, r.pool)
	var entity domaintrip.Trip
	err = querier.QueryRow(ctx, query, args...).Scan(
		&entity.ID, &entity.UserID, &entity.DriverID, &entity.StartLatitude, &entity.StartLongitude,
		&entity.EndLatitude, &entity.EndLongitude, &entity.Price, &entity.Status, &entity.StartedAt,
		&entity.FinishedAt, &entity.LastPositionAt, &entity.CreatedAt, &entity.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaintrip.Trip{}, domaintrip.ErrTripNotFound
	}
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("select trip: %w", err)
	}
	return entity, nil
}

func (r *Repository) UpdateLastPositionAt(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error {
	query, args, err := psql.
		Update("trips").
		Set("last_position_at", sq.Expr("GREATEST(COALESCE(last_position_at, ?), ?)", recordedAt, recordedAt)).
		Set("updated_at", sq.Expr("now()")).
		Where(sq.Eq{"id": tripID, statusColumn: domaintrip.StatusActive}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update last position: %w", err)
	}

	querier := transaction.QuerierFromContext(ctx, r.pool)
	command, err := querier.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update last position: %w", err)
	}
	if command.RowsAffected() == 1 {
		return nil
	}

	entity, err := r.GetByID(ctx, tripID)
	if err != nil {
		return err
	}
	if entity.Status == domaintrip.StatusCompleted {
		return domaintrip.ErrTripCompleted
	}
	return domaintrip.ErrTripNotFound
}

func (r *Repository) InsertPosition(ctx context.Context, entity *domaintrip.Position) error {
	query, args, err := psql.
		Insert("trip_positions").
		Columns("trip_id", "latitude", "longitude", "recorded_at", "created_at").
		Values(entity.TripID, entity.Latitude, entity.Longitude, entity.RecordedAt, entity.CreatedAt).
		Suffix("RETURNING id").
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert position: %w", err)
	}

	querier := transaction.QuerierFromContext(ctx, r.pool)
	if err := querier.QueryRow(ctx, query, args...).Scan(&entity.ID); err != nil {
		return fmt.Errorf("insert position: %w", err)
	}
	return nil
}

func (r *Repository) ListPositions(ctx context.Context, tripID uuid.UUID) ([]domaintrip.Position, error) {
	query, args, err := psql.
		Select("id", "trip_id", "latitude", "longitude", "recorded_at", "created_at").
		From("trip_positions").
		Where(sq.Eq{"trip_id": tripID}).
		OrderBy("recorded_at ASC", "id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list positions: %w", err)
	}

	querier := transaction.QuerierFromContext(ctx, r.pool)
	rows, err := querier.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select positions: %w", err)
	}
	defer rows.Close()

	positions := make([]domaintrip.Position, 0)
	for rows.Next() {
		var entity domaintrip.Position
		if err := rows.Scan(
			&entity.ID, &entity.TripID, &entity.Latitude, &entity.Longitude,
			&entity.RecordedAt, &entity.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan position: %w", err)
		}
		positions = append(positions, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate positions: %w", err)
	}
	return positions, nil
}

func (r *Repository) ListStaleTrips(ctx context.Context, staleAfter time.Duration) ([]domaintrip.StaleTrip, error) {
	const query = `
		SELECT id, driver_id, COALESCE(last_position_at, started_at) AS stale_since
		FROM trips
		WHERE status = 'active'
		  AND COALESCE(last_position_at, started_at) < now() - ($1 * interval '1 second')
		ORDER BY COALESCE(last_position_at, started_at) ASC, id ASC`

	querier := transaction.QuerierFromContext(ctx, r.pool)
	rows, err := querier.Query(ctx, query, staleAfter.Seconds())
	if err != nil {
		return nil, fmt.Errorf("select stale trips: %w", err)
	}
	defer rows.Close()

	trips := make([]domaintrip.StaleTrip, 0)
	for rows.Next() {
		var entity domaintrip.StaleTrip
		if err := rows.Scan(&entity.TripID, &entity.DriverID, &entity.StaleSince); err != nil {
			return nil, fmt.Errorf("scan stale trip: %w", err)
		}
		trips = append(trips, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale trips: %w", err)
	}
	return trips, nil
}

func (r *Repository) Finish(ctx context.Context, entity domaintrip.Trip) error {
	query, args, err := psql.
		Update("trips").
		Set("status", entity.Status).
		Set("finished_at", entity.FinishedAt).
		Set("updated_at", entity.UpdatedAt).
		Where(sq.Eq{"id": entity.ID, statusColumn: domaintrip.StatusActive}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build finish trip: %w", err)
	}

	querier := transaction.QuerierFromContext(ctx, r.pool)
	command, err := querier.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update trip: %w", err)
	}
	if command.RowsAffected() == 1 {
		return nil
	}

	existing, err := r.GetByID(ctx, entity.ID)
	if err != nil {
		return err
	}
	if existing.Status == domaintrip.StatusCompleted {
		return domaintrip.ErrTripCompleted
	}
	return domaintrip.ErrTripNotFound
}

func (r *Repository) InsertStatus(
	ctx context.Context,
	tripID uuid.UUID,
	fromStatus *domaintrip.Status,
	toStatus domaintrip.Status,
	reason string,
	changedAt time.Time,
) error {
	query, args, err := psql.
		Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason", "changed_at").
		Values(tripID, fromStatus, toStatus, reason, changedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert status history: %w", err)
	}

	querier := transaction.QuerierFromContext(ctx, r.pool)
	if _, err := querier.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert status history: %w", err)
	}
	return nil
}

func (r *Repository) CountActive(ctx context.Context) (int64, error) {
	query, args, err := psql.
		Select("COUNT(*)").
		From("trips").
		Where(sq.Eq{statusColumn: domaintrip.StatusActive}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count active trips: %w", err)
	}

	querier := transaction.QuerierFromContext(ctx, r.pool)
	var count int64
	if err := querier.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active trips: %w", err)
	}
	return count, nil
}
