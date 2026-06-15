package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"office_trip/internal/model"
)

type TripRepo struct {
	db *sqlx.DB
}

func NewTripRepo(db *sqlx.DB) *TripRepo {
	return &TripRepo{db: db}
}

func (r *TripRepo) Create(ctx context.Context, t *model.Trip) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx, `
		INSERT INTO trips (id, driver_id, office_id, zone_id, origin, origin_address, depart_at,
		                   seats_total, seats_left, route_geojson, duration_seconds, distance_meters, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($6, $5), 4326), $7, $8, $9, $10, $11, $12, $13, 'scheduled', NOW(), NOW())
	`,
		t.ID, t.DriverID, t.OfficeID, t.ZoneID,
		t.OriginLat, t.OriginLng, t.OriginAddress,
		t.DepartAt, t.SeatsTotal, t.SeatsLeft,
		t.RouteGeoJSON, t.DurationSeconds, t.DistanceMeters,
	)
	return err
}

func (r *TripRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	t, err := r.scanTrip(ctx, getDbRunner(ctx, r.db), id, false)
	if err != nil {
		return nil, err
	}
	stops, err := r.GetStopsByTripID(ctx, id)
	if err == nil {
		t.Stops = stops
	}
	return t, nil
}

func (r *TripRepo) GetForUpdate(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	t, err := r.scanTrip(ctx, getDbRunner(ctx, r.db), id, true)
	if err != nil {
		return nil, err
	}
	stops, err := r.GetStopsByTripID(ctx, id)
	if err == nil {
		t.Stops = stops
	}
	return t, nil
}

type dbQuerier interface {
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
}

func (r *TripRepo) scanTrip(ctx context.Context, q dbQuerier, id uuid.UUID, forUpdate bool) (*model.Trip, error) {
	var row struct {
		ID              uuid.UUID        `db:"id"`
		DriverID        uuid.UUID        `db:"driver_id"`
		OfficeID        uuid.UUID        `db:"office_id"`
		ZoneID          *uuid.UUID       `db:"zone_id"`
		Lat             float64          `db:"lat"`
		Lng             float64          `db:"lng"`
		OriginAddress   string           `db:"origin_address"`
		DepartAt        time.Time        `db:"depart_at"`
		SeatsTotal      int              `db:"seats_total"`
		SeatsLeft       int              `db:"seats_left"`
		RouteGeoJSON    string           `db:"route_geojson"`
		DurationSeconds int              `db:"duration_seconds"`
		DistanceMeters  int              `db:"distance_meters"`
		Status          model.TripStatus `db:"status"`
		CreatedAt       time.Time        `db:"created_at"`
		UpdatedAt       time.Time        `db:"updated_at"`
		DriverName      string           `db:"driver_name"`
		OfficeName      string           `db:"office_name"`
	}

	lock := ""
	if forUpdate {
		lock = "FOR UPDATE"
	}

	query := `
		SELECT t.id, t.driver_id, t.office_id, t.zone_id,
		       ST_Y(t.origin::geometry) AS lat,
		       ST_X(t.origin::geometry) AS lng,
		       t.origin_address, t.depart_at, t.seats_total, t.seats_left,
		       t.route_geojson, t.duration_seconds, t.distance_meters, t.status,
		       t.created_at, t.updated_at,
		       u.full_name AS driver_name,
		       o.name AS office_name
		FROM trips t
		JOIN users u ON u.id = t.driver_id
		JOIN offices o ON o.id = t.office_id
		WHERE t.id = $1 ` + lock

	err := q.GetContext(ctx, &row, query, id)
	if err == sql.ErrNoRows {
		return nil, model.ErrTripNotFound
	}
	if err != nil {
		return nil, err
	}

	return &model.Trip{
		ID:              row.ID,
		DriverID:        row.DriverID,
		OfficeID:        row.OfficeID,
		ZoneID:          row.ZoneID,
		OriginLat:       row.Lat,
		OriginLng:       row.Lng,
		OriginAddress:   row.OriginAddress,
		DepartAt:        row.DepartAt,
		SeatsTotal:      row.SeatsTotal,
		SeatsLeft:       row.SeatsLeft,
		RouteGeoJSON:    row.RouteGeoJSON,
		DurationSeconds: row.DurationSeconds,
		DistanceMeters:  row.DistanceMeters,
		Status:          row.Status,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		DriverName:      row.DriverName,
		OfficeName:      row.OfficeName,
	}, nil
}

func (r *TripRepo) List(ctx context.Context, officeID uuid.UUID, date time.Time, lat, lng float64, maxDist float64, limit, offset int) ([]*model.Trip, int, error) {
	rows, err := getDbRunner(ctx, r.db).QueryxContext(ctx, `
		SELECT t.id, t.driver_id, t.office_id, t.zone_id,
		       ST_Y(t.origin::geometry) AS lat,
		       ST_X(t.origin::geometry) AS lng,
		       t.origin_address, t.depart_at, t.seats_total, t.seats_left,
		       t.route_geojson, t.duration_seconds, t.distance_meters, t.status,
		       t.created_at, t.updated_at,
		       u.full_name AS driver_name,
		       o.name AS office_name,
		       CASE 
		           WHEN $3 = 0.0 AND $4 = 0.0 THEN 0.0
		           ELSE ST_Distance(
		               ST_SetSRID(ST_MakePoint($4, $3), 4326)::geography,
		               ST_GeomFromGeoJSON(t.route_geojson)::geography
		           )
		       END AS distance_to_route
		FROM trips t
		JOIN users u ON u.id = t.driver_id
		JOIN offices o ON o.id = t.office_id
		WHERE ($1 = '00000000-0000-0000-0000-000000000000'::uuid OR t.office_id = $1)
		  AND t.depart_at::date = $2::date
		  AND t.status = 'scheduled'
		  AND t.seats_left > 0
		  AND t.depart_at > NOW() + INTERVAL '30 minutes'
		  AND (
		      ($3 = 0.0 AND $4 = 0.0)
		      OR
		      ST_DWithin(
		          ST_SetSRID(ST_MakePoint($4, $3), 4326)::geography,
		          ST_GeomFromGeoJSON(t.route_geojson)::geography,
		          $5
		      )
		  )
		ORDER BY t.depart_at ASC, distance_to_route ASC
		LIMIT $6 OFFSET $7
	`, officeID, date, lat, lng, maxDist, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var trips []*model.Trip
	for rows.Next() {
		var row struct {
			ID              uuid.UUID        `db:"id"`
			DriverID        uuid.UUID        `db:"driver_id"`
			OfficeID        uuid.UUID        `db:"office_id"`
			ZoneID          *uuid.UUID       `db:"zone_id"`
			Lat             float64          `db:"lat"`
			Lng             float64          `db:"lng"`
			OriginAddress   string           `db:"origin_address"`
			DepartAt        time.Time        `db:"depart_at"`
			SeatsTotal      int              `db:"seats_total"`
			SeatsLeft       int              `db:"seats_left"`
			RouteGeoJSON    string           `db:"route_geojson"`
			DurationSeconds int              `db:"duration_seconds"`
			DistanceMeters  int              `db:"distance_meters"`
			Status          model.TripStatus `db:"status"`
			CreatedAt       time.Time        `db:"created_at"`
			UpdatedAt       time.Time        `db:"updated_at"`
			DriverName      string           `db:"driver_name"`
			OfficeName      string           `db:"office_name"`
			DistanceToRoute float64          `db:"distance_to_route"`
		}
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, err
		}
		trips = append(trips, &model.Trip{
			ID:              row.ID,
			DriverID:        row.DriverID,
			OfficeID:        row.OfficeID,
			ZoneID:          row.ZoneID,
			OriginLat:       row.Lat,
			OriginLng:       row.Lng,
			OriginAddress:   row.OriginAddress,
			DepartAt:        row.DepartAt,
			SeatsTotal:      row.SeatsTotal,
			SeatsLeft:       row.SeatsLeft,
			RouteGeoJSON:    row.RouteGeoJSON,
			DurationSeconds: row.DurationSeconds,
			DistanceMeters:  row.DistanceMeters,
			Status:          row.Status,
			CreatedAt:       row.CreatedAt,
			UpdatedAt:       row.UpdatedAt,
			DriverName:      row.DriverName,
			OfficeName:      row.OfficeName,
			DistanceToRoute: row.DistanceToRoute,
		})
	}

	var total int
	_ = getDbRunner(ctx, r.db).GetContext(ctx, &total, `
		SELECT COUNT(*) FROM trips t
		WHERE ($1 = '00000000-0000-0000-0000-000000000000'::uuid OR t.office_id = $1)
		  AND t.depart_at::date = $2::date
		  AND t.status = 'scheduled'
		  AND t.seats_left > 0
		  AND t.depart_at > NOW() + INTERVAL '30 minutes'
	`, officeID, date)

	return trips, total, nil
}

func (r *TripRepo) ListByDriver(ctx context.Context, driverID uuid.UUID, limit, offset int) ([]*model.Trip, int, error) {
	var rows []struct {
		ID              uuid.UUID        `db:"id"`
		DriverID        uuid.UUID        `db:"driver_id"`
		OfficeID        uuid.UUID        `db:"office_id"`
		ZoneID          *uuid.UUID       `db:"zone_id"`
		Lat             float64          `db:"lat"`
		Lng             float64          `db:"lng"`
		OriginAddress   string           `db:"origin_address"`
		DepartAt        time.Time        `db:"depart_at"`
		SeatsTotal      int              `db:"seats_total"`
		SeatsLeft       int              `db:"seats_left"`
		RouteGeoJSON    string           `db:"route_geojson"`
		DurationSeconds int              `db:"duration_seconds"`
		DistanceMeters  int              `db:"distance_meters"`
		Status          model.TripStatus `db:"status"`
		CreatedAt       time.Time        `db:"created_at"`
		UpdatedAt       time.Time        `db:"updated_at"`
		OfficeName      string           `db:"office_name"`
	}

	err := getDbRunner(ctx, r.db).SelectContext(ctx, &rows, `
		SELECT t.id, t.driver_id, t.office_id, t.zone_id,
		       ST_Y(t.origin::geometry) AS lat,
		       ST_X(t.origin::geometry) AS lng,
		       t.origin_address, t.depart_at, t.seats_total, t.seats_left,
		       t.route_geojson, t.duration_seconds, t.distance_meters, t.status,
		       t.created_at, t.updated_at,
		       o.name AS office_name
		FROM trips t
		JOIN offices o ON o.id = t.office_id
		WHERE t.driver_id = $1
		ORDER BY t.depart_at DESC
		LIMIT $2 OFFSET $3
	`, driverID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	trips := make([]*model.Trip, len(rows))
	for i, row := range rows {
		trips[i] = &model.Trip{
			ID: row.ID, DriverID: row.DriverID, OfficeID: row.OfficeID, ZoneID: row.ZoneID,
			OriginLat: row.Lat, OriginLng: row.Lng, OriginAddress: row.OriginAddress,
			DepartAt: row.DepartAt, SeatsTotal: row.SeatsTotal, SeatsLeft: row.SeatsLeft,
			RouteGeoJSON: row.RouteGeoJSON, DurationSeconds: row.DurationSeconds,
			DistanceMeters: row.DistanceMeters, Status: row.Status,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, OfficeName: row.OfficeName,
		}
	}

	var total int
	_ = getDbRunner(ctx, r.db).GetContext(ctx, &total, `SELECT COUNT(*) FROM trips WHERE driver_id = $1`, driverID)
	return trips, total, nil
}

func (r *TripRepo) ListJoined(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*model.Trip, int, error) {
	var rows []struct {
		ID              uuid.UUID        `db:"id"`
		DriverID        uuid.UUID        `db:"driver_id"`
		OfficeID        uuid.UUID        `db:"office_id"`
		ZoneID          *uuid.UUID       `db:"zone_id"`
		Lat             float64          `db:"lat"`
		Lng             float64          `db:"lng"`
		OriginAddress   string           `db:"origin_address"`
		DepartAt        time.Time        `db:"depart_at"`
		SeatsTotal      int              `db:"seats_total"`
		SeatsLeft       int              `db:"seats_left"`
		RouteGeoJSON    string           `db:"route_geojson"`
		DurationSeconds int              `db:"duration_seconds"`
		DistanceMeters  int              `db:"distance_meters"`
		Status          model.TripStatus `db:"status"`
		CreatedAt       time.Time        `db:"created_at"`
		UpdatedAt       time.Time        `db:"updated_at"`
		DriverName      string           `db:"driver_name"`
		OfficeName      string           `db:"office_name"`
	}

	err := getDbRunner(ctx, r.db).SelectContext(ctx, &rows, `
		SELECT t.id, t.driver_id, t.office_id, t.zone_id,
		       ST_Y(t.origin::geometry) AS lat,
		       ST_X(t.origin::geometry) AS lng,
		       t.origin_address, t.depart_at, t.seats_total, t.seats_left,
		       t.route_geojson, t.duration_seconds, t.distance_meters, t.status,
		       t.created_at, t.updated_at,
		       u.full_name AS driver_name,
		       o.name AS office_name
		FROM trips t
		JOIN users u ON u.id = t.driver_id
		JOIN offices o ON o.id = t.office_id
		JOIN trip_passengers tp ON tp.trip_id = t.id
		WHERE tp.user_id = $1 AND tp.status = 'confirmed'
		ORDER BY t.depart_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	trips := make([]*model.Trip, len(rows))
	for i, row := range rows {
		trips[i] = &model.Trip{
			ID: row.ID, DriverID: row.DriverID, OfficeID: row.OfficeID, ZoneID: row.ZoneID,
			OriginLat: row.Lat, OriginLng: row.Lng, OriginAddress: row.OriginAddress,
			DepartAt: row.DepartAt, SeatsTotal: row.SeatsTotal, SeatsLeft: row.SeatsLeft,
			RouteGeoJSON: row.RouteGeoJSON, DurationSeconds: row.DurationSeconds,
			DistanceMeters: row.DistanceMeters, Status: row.Status,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			DriverName: row.DriverName, OfficeName: row.OfficeName,
		}
	}

	var total int
	_ = getDbRunner(ctx, r.db).GetContext(ctx, &total, `
		SELECT COUNT(*) FROM trip_passengers tp
		WHERE tp.user_id = $1 AND tp.status = 'confirmed'
	`, userID)
	return trips, total, nil
}

func (r *TripRepo) ListAll(ctx context.Context, limit, offset int) ([]*model.Trip, int, error) {
	var rows []struct {
		ID              uuid.UUID        `db:"id"`
		DriverID        uuid.UUID        `db:"driver_id"`
		OfficeID        uuid.UUID        `db:"office_id"`
		ZoneID          *uuid.UUID       `db:"zone_id"`
		Lat             float64          `db:"lat"`
		Lng             float64          `db:"lng"`
		OriginAddress   string           `db:"origin_address"`
		DepartAt        time.Time        `db:"depart_at"`
		SeatsTotal      int              `db:"seats_total"`
		SeatsLeft       int              `db:"seats_left"`
		RouteGeoJSON    string           `db:"route_geojson"`
		DurationSeconds int              `db:"duration_seconds"`
		DistanceMeters  int              `db:"distance_meters"`
		Status          model.TripStatus `db:"status"`
		CreatedAt       time.Time        `db:"created_at"`
		UpdatedAt       time.Time        `db:"updated_at"`
		DriverName      string           `db:"driver_name"`
		OfficeName      string           `db:"office_name"`
	}

	err := getDbRunner(ctx, r.db).SelectContext(ctx, &rows, `
		SELECT t.id, t.driver_id, t.office_id, t.zone_id,
		       ST_Y(t.origin::geometry) AS lat,
		       ST_X(t.origin::geometry) AS lng,
		       t.origin_address, t.depart_at, t.seats_total, t.seats_left,
		       t.route_geojson, t.duration_seconds, t.distance_meters, t.status,
		       t.created_at, t.updated_at,
		       u.full_name AS driver_name,
		       o.name AS office_name
		FROM trips t
		JOIN users u ON u.id = t.driver_id
		JOIN offices o ON o.id = t.office_id
		ORDER BY t.depart_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	trips := make([]*model.Trip, len(rows))
	for i, row := range rows {
		trips[i] = &model.Trip{
			ID: row.ID, DriverID: row.DriverID, OfficeID: row.OfficeID, ZoneID: row.ZoneID,
			OriginLat: row.Lat, OriginLng: row.Lng, OriginAddress: row.OriginAddress,
			DepartAt: row.DepartAt, SeatsTotal: row.SeatsTotal, SeatsLeft: row.SeatsLeft,
			RouteGeoJSON: row.RouteGeoJSON, DurationSeconds: row.DurationSeconds,
			DistanceMeters: row.DistanceMeters, Status: row.Status,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			DriverName: row.DriverName, OfficeName: row.OfficeName,
		}
	}

	var total int
	_ = getDbRunner(ctx, r.db).GetContext(ctx, &total, `SELECT COUNT(*) FROM trips`)
	return trips, total, nil
}

func (r *TripRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status model.TripStatus) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx,
		`UPDATE trips SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	return err
}

func (r *TripRepo) HasDriverConflict(ctx context.Context, driverID uuid.UUID, departAt time.Time) (bool, error) {
	var count int
	err := getDbRunner(ctx, r.db).GetContext(ctx, &count, `
		SELECT COUNT(*) FROM trips
		WHERE driver_id = $1
		  AND status IN ('scheduled', 'in_progress')
		  AND depart_at BETWEEN ($2::timestamptz - INTERVAL '2 hours')
		                    AND ($2::timestamptz + INTERVAL '2 hours')
	`, driverID, departAt)
	return count > 0, err
}

func (r *TripRepo) HasPassengerConflict(ctx context.Context, passengerID uuid.UUID, departAt time.Time) (bool, error) {
	var count int
	err := getDbRunner(ctx, r.db).GetContext(ctx, &count, `
		SELECT COUNT(*) FROM trip_passengers tp
		JOIN trips t ON t.id = tp.trip_id
		WHERE tp.user_id = $1
		  AND tp.status = 'confirmed'
		  AND t.status = 'scheduled'
		  AND t.depart_at BETWEEN ($2::timestamptz - INTERVAL '2 hours')
		                      AND ($2::timestamptz + INTERVAL '2 hours')
	`, passengerID, departAt)
	return count > 0, err
}

func (r *TripRepo) GetDistanceToRoute(ctx context.Context, tripID uuid.UUID, lat, lng float64) (float64, error) {
	var dist float64
	err := getDbRunner(ctx, r.db).GetContext(ctx, &dist, `
		SELECT ST_Distance(
		    ST_SetSRID(ST_MakePoint($3, $2), 4326)::geography,
		    ST_GeomFromGeoJSON(route_geojson)::geography
		) FROM trips WHERE id = $1
	`, tripID, lat, lng)
	return dist, err
}

func (r *TripRepo) IsPassengerInTrip(ctx context.Context, tripID, userID uuid.UUID) (bool, error) {
	var count int
	err := getDbRunner(ctx, r.db).GetContext(ctx, &count, `
		SELECT COUNT(*) FROM trip_passengers
		WHERE trip_id = $1 AND user_id = $2 AND status = 'confirmed'
	`, tripID, userID)
	return count > 0, err
}

func (r *TripRepo) AddPassenger(ctx context.Context, p *model.TripPassenger) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx, `
		INSERT INTO trip_passengers (id, trip_id, user_id, pickup, pickup_address, detour_seconds, status, joined_at, stop_id)
		VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($5, $4), 4326), $6, $7, 'confirmed', NOW(), $8)
	`, p.ID, p.TripID, p.UserID, p.PickupLat, p.PickupLng, p.PickupAddress, p.DetourSeconds, p.StopID)
	return err
}

func (r *TripRepo) DecrementSeats(ctx context.Context, tripID uuid.UUID) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx,
		`UPDATE trips SET seats_left = seats_left - 1, updated_at = NOW() WHERE id = $1 AND seats_left > 0`,
		tripID)
	return err
}

func (r *TripRepo) IncrementSeats(ctx context.Context, tripID uuid.UUID) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx,
		`UPDATE trips SET seats_left = seats_left + 1, updated_at = NOW() WHERE id = $1 AND seats_left < seats_total`,
		tripID)
	return err
}

func (r *TripRepo) GetPassenger(ctx context.Context, tripID, userID uuid.UUID) (*model.TripPassenger, error) {
	p := &model.TripPassenger{}
	err := getDbRunner(ctx, r.db).GetContext(ctx, p, `
		SELECT id, trip_id, user_id, pickup_address, detour_seconds, status, joined_at, cancelled_at, stop_id,
		       ST_Y(pickup::geometry) AS pickup_lat,
		       ST_X(pickup::geometry) AS pickup_lng
		FROM trip_passengers
		WHERE trip_id = $1 AND user_id = $2
	`, tripID, userID)
	if err == sql.ErrNoRows {
		return nil, model.ErrUserNotFound
	}
	return p, err
}

func (r *TripRepo) CancelPassenger(ctx context.Context, tripID, userID uuid.UUID) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx, `
		UPDATE trip_passengers
		SET status = 'cancelled', cancelled_at = NOW()
		WHERE trip_id = $1 AND user_id = $2 AND status = 'confirmed'
	`, tripID, userID)
	return err
}

func (r *TripRepo) UpdatePassengerStop(ctx context.Context, tripID, userID, stopID uuid.UUID, address string, lat, lng float64) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx, `
		UPDATE trip_passengers
		SET stop_id = $1, pickup_address = $2, pickup = ST_SetSRID(ST_MakePoint($4, $3), 4326)
		WHERE trip_id = $5 AND user_id = $6 AND status = 'confirmed'
	`, stopID, address, lat, lng, tripID, userID)
	return err
}

func (r *TripRepo) GetPassengers(ctx context.Context, tripID uuid.UUID) ([]*model.TripPassenger, error) {
	var rows []struct {
		ID            uuid.UUID  `db:"id"`
		TripID        uuid.UUID  `db:"trip_id"`
		UserID        uuid.UUID  `db:"user_id"`
		PickupAddress string     `db:"pickup_address"`
		DetourSeconds int        `db:"detour_seconds"`
		Status        string     `db:"status"`
		JoinedAt      time.Time  `db:"joined_at"`
		CancelledAt   *time.Time `db:"cancelled_at"`
		StopID        *uuid.UUID `db:"stop_id"`
		PickupLat     float64    `db:"pickup_lat"`
		PickupLng     float64    `db:"pickup_lng"`
		UserFullName  string     `db:"user_full_name"`
	}

	err := getDbRunner(ctx, r.db).SelectContext(ctx, &rows, `
		SELECT tp.id, tp.trip_id, tp.user_id, tp.pickup_address, tp.detour_seconds,
		       tp.status, tp.joined_at, tp.cancelled_at, tp.stop_id,
		       ST_Y(tp.pickup::geometry) AS pickup_lat,
		       ST_X(tp.pickup::geometry) AS pickup_lng,
		       u.full_name AS user_full_name
		FROM trip_passengers tp
		JOIN users u ON u.id = tp.user_id
		WHERE tp.trip_id = $1 AND tp.status = 'confirmed'
		ORDER BY tp.joined_at ASC
	`, tripID)
	if err != nil {
		return nil, err
	}

	passengers := make([]*model.TripPassenger, len(rows))
	for i, row := range rows {
		passengers[i] = &model.TripPassenger{
			ID:            row.ID,
			TripID:        row.TripID,
			UserID:        row.UserID,
			PickupAddress: row.PickupAddress,
			DetourSeconds: row.DetourSeconds,
			Status:        row.Status,
			JoinedAt:      row.JoinedAt,
			CancelledAt:   row.CancelledAt,
			StopID:        row.StopID,
			PickupLat:     row.PickupLat,
			PickupLng:     row.PickupLng,
			UserFullName:  row.UserFullName,
		}
	}
	return passengers, nil
}

func (r *TripRepo) GetPassengerUserIDs(ctx context.Context, tripID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := getDbRunner(ctx, r.db).SelectContext(ctx, &ids, `
		SELECT user_id FROM trip_passengers
		WHERE trip_id = $1 AND status = 'confirmed'
	`, tripID)
	return ids, err
}

func (r *TripRepo) CreateStop(ctx context.Context, s *model.TripStop) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx, `
		INSERT INTO trip_stops (id, trip_id, location, address, arrival_time, sequence_number)
		VALUES ($1, $2, ST_SetSRID(ST_MakePoint($4, $3), 4326), $5, $6, $7)
	`, s.ID, s.TripID, s.Lat, s.Lng, s.Address, s.ArrivalTime, s.SequenceNumber)
	return err
}

func (r *TripRepo) GetStopsByTripID(ctx context.Context, tripID uuid.UUID) ([]*model.TripStop, error) {
	var rows []struct {
		ID             uuid.UUID `db:"id"`
		TripID         uuid.UUID `db:"trip_id"`
		Lat            float64   `db:"lat"`
		Lng            float64   `db:"lng"`
		Address        string    `db:"address"`
		ArrivalTime    time.Time `db:"arrival_time"`
		SequenceNumber int       `db:"sequence_number"`
	}
	err := getDbRunner(ctx, r.db).SelectContext(ctx, &rows, `
		SELECT id, trip_id, ST_Y(location::geometry) AS lat, ST_X(location::geometry) AS lng,
		       address, arrival_time, sequence_number
		FROM trip_stops
		WHERE trip_id = $1
		ORDER BY sequence_number ASC
	`, tripID)
	if err != nil {
		return nil, err
	}
	stops := make([]*model.TripStop, len(rows))
	for i, r := range rows {
		stops[i] = &model.TripStop{
			ID: r.ID, TripID: r.TripID, Lat: r.Lat, Lng: r.Lng,
			Address: r.Address, ArrivalTime: r.ArrivalTime, SequenceNumber: r.SequenceNumber,
		}
	}
	return stops, nil
}

func (r *TripRepo) GetStopByID(ctx context.Context, id uuid.UUID) (*model.TripStop, error) {
	var row struct {
		ID             uuid.UUID `db:"id"`
		TripID         uuid.UUID `db:"trip_id"`
		Lat            float64   `db:"lat"`
		Lng            float64   `db:"lng"`
		Address        string    `db:"address"`
		ArrivalTime    time.Time `db:"arrival_time"`
		SequenceNumber int       `db:"sequence_number"`
	}
	err := getDbRunner(ctx, r.db).GetContext(ctx, &row, `
		SELECT id, trip_id, ST_Y(location::geometry) AS lat, ST_X(location::geometry) AS lng,
		       address, arrival_time, sequence_number
		FROM trip_stops
		WHERE id = $1
	`, id)
	if err == sql.ErrNoRows {
		return nil, model.ErrStopNotFound
	}
	if err != nil {
		return nil, err
	}
	return &model.TripStop{
		ID: row.ID, TripID: row.TripID, Lat: row.Lat, Lng: row.Lng,
		Address: row.Address, ArrivalTime: row.ArrivalTime, SequenceNumber: row.SequenceNumber,
	}, nil
}
