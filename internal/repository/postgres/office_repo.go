package postgres

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"office_trip/internal/dto"
	"office_trip/internal/model"
)

type OfficeRepo struct {
	db *sqlx.DB
}

func NewOfficeRepo(db *sqlx.DB) *OfficeRepo {
	return &OfficeRepo{db: db}
}

//row для выборки из бд с st asgeojson для геометрии
type officeRow struct {
	ID          uuid.UUID `db:"id"`
	CompanyID   uuid.UUID `db:"company_id"`
	Name        string    `db:"name"`
	Address     string    `db:"address"`
	Lat         float64   `db:"lat"`
	Lng         float64   `db:"lng"`
	IsActive    bool      `db:"is_active"`
}

func (r *OfficeRepo) Create(ctx context.Context, o *model.Office) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx, `
		INSERT INTO offices (id, company_id, name, address, location, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($6, $5), 4326), $7, NOW(), NOW())
	`, o.ID, o.CompanyID, o.Name, o.Address, o.Lat, o.Lng, o.IsActive)
	return err
}

func (r *OfficeRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Office, error) {
	var row struct {
		ID        uuid.UUID `db:"id"`
		CompanyID uuid.UUID `db:"company_id"`
		Name      string    `db:"name"`
		Address   string    `db:"address"`
		Lat       float64   `db:"lat"`
		Lng       float64   `db:"lng"`
		IsActive  bool      `db:"is_active"`
	}
	err := getDbRunner(ctx, r.db).GetContext(ctx, &row, `
		SELECT id, company_id, name, address, is_active,
		       ST_Y(location::geometry) AS lat,
		       ST_X(location::geometry) AS lng
		FROM offices
		WHERE id = $1
	`, id)
	if err == sql.ErrNoRows {
		return nil, model.ErrOfficeNotFound
	}
	if err != nil {
		return nil, err
	}

	o := &model.Office{
		ID:        row.ID,
		CompanyID: row.CompanyID,
		Name:      row.Name,
		Address:   row.Address,
		Lat:       row.Lat,
		Lng:       row.Lng,
		IsActive:  row.IsActive,
	}

	//получение зоны при наличии
	zone, err := r.GetZoneByOffice(ctx, id)
	if err == nil && zone != nil {
		o.ZoneGeoJSON = zone.PickupZoneGeoJSON
	}

	return o, nil
}

func (r *OfficeRepo) List(ctx context.Context, companyID uuid.UUID) ([]*model.Office, error) {
	rows, err := getDbRunner(ctx, r.db).QueryxContext(ctx, `
		SELECT id, company_id, name, address, is_active,
		       ST_Y(location::geometry) AS lat,
		       ST_X(location::geometry) AS lng
		FROM offices
		WHERE company_id = $1
		ORDER BY name ASC
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var offices []*model.Office
	for rows.Next() {
		var row struct {
			ID        uuid.UUID `db:"id"`
			CompanyID uuid.UUID `db:"company_id"`
			Name      string    `db:"name"`
			Address   string    `db:"address"`
			IsActive  bool      `db:"is_active"`
			Lat       float64   `db:"lat"`
			Lng       float64   `db:"lng"`
		}
		if err := rows.StructScan(&row); err != nil {
			return nil, err
		}
		offices = append(offices, &model.Office{
			ID:        row.ID,
			CompanyID: row.CompanyID,
			Name:      row.Name,
			Address:   row.Address,
			Lat:       row.Lat,
			Lng:       row.Lng,
			IsActive:  row.IsActive,
		})
	}
	return offices, nil
}

func (r *OfficeRepo) Update(ctx context.Context, o *model.Office) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx, `
		UPDATE offices SET name = $1, address = $2,
		       location = ST_SetSRID(ST_MakePoint($4, $3), 4326),
		       updated_at = NOW()
		WHERE id = $5
	`, o.Name, o.Address, o.Lat, o.Lng, o.ID)
	return err
}

func (r *OfficeRepo) Deactivate(ctx context.Context, id uuid.UUID) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx,
		`UPDATE offices SET is_active = false, updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *OfficeRepo) CreateZone(ctx context.Context, zone *model.OfficeZone, geoJSONStr string) error {
	_, err := getDbRunner(ctx, r.db).ExecContext(ctx, `
		INSERT INTO office_zones (id, office_id, pickup_zone, max_detour_minutes, max_distance_meters, min_join_minutes, min_cancel_minutes, max_seats, is_active, created_at, updated_at)
		VALUES ($1, $2, ST_GeomFromGeoJSON($3), $4, $5, $6, $7, $8, $9, NOW(), NOW())
	`,
		zone.ID, zone.OfficeID, geoJSONStr,
		zone.MaxDetourMinutes, zone.MaxDistanceMeters,
		zone.MinJoinMinutes, zone.MinCancelMinutes,
		zone.MaxSeats, zone.IsActive,
	)
	return err
}

func (r *OfficeRepo) GetZoneByOffice(ctx context.Context, officeID uuid.UUID) (*model.OfficeZone, error) {
	var zone struct {
		ID                uuid.UUID `db:"id"`
		OfficeID          uuid.UUID `db:"office_id"`
		PickupZoneGeoJSON string    `db:"pickup_zone_geojson"`
		MaxDetourMinutes  int       `db:"max_detour_minutes"`
		MaxDistanceMeters int       `db:"max_distance_meters"`
		MinJoinMinutes    int       `db:"min_join_minutes"`
		MinCancelMinutes  int       `db:"min_cancel_minutes"`
		MaxSeats          int       `db:"max_seats"`
		IsActive          bool      `db:"is_active"`
	}
	err := getDbRunner(ctx, r.db).GetContext(ctx, &zone, `
		SELECT id, office_id,
		       ST_AsGeoJSON(pickup_zone) AS pickup_zone_geojson,
		       max_detour_minutes, max_distance_meters,
		       min_join_minutes, min_cancel_minutes, max_seats, is_active
		FROM office_zones
		WHERE office_id = $1 AND is_active = true
		LIMIT 1
	`, officeID)
	if err == sql.ErrNoRows {
		return nil, model.ErrZoneNotFound
	}
	if err != nil {
		return nil, err
	}
	return &model.OfficeZone{
		ID:                zone.ID,
		OfficeID:          zone.OfficeID,
		PickupZoneGeoJSON: zone.PickupZoneGeoJSON,
		MaxDetourMinutes:  zone.MaxDetourMinutes,
		MaxDistanceMeters: zone.MaxDistanceMeters,
		MinJoinMinutes:    zone.MinJoinMinutes,
		MinCancelMinutes:  zone.MinCancelMinutes,
		MaxSeats:          zone.MaxSeats,
		IsActive:          zone.IsActive,
	}, nil
}

func (r *OfficeRepo) UpdateZone(ctx context.Context, zoneID uuid.UUID, req dto.UpdateZoneRequest) error {
	runner := getDbRunner(ctx, r.db)
	if req.MaxDetourMinutes != nil {
		if _, err := runner.ExecContext(ctx, `UPDATE office_zones SET max_detour_minutes = $1, updated_at = NOW() WHERE id = $2`, *req.MaxDetourMinutes, zoneID); err != nil {
			return err
		}
	}
	if req.MaxDistanceMeters != nil {
		if _, err := runner.ExecContext(ctx, `UPDATE office_zones SET max_distance_meters = $1, updated_at = NOW() WHERE id = $2`, *req.MaxDistanceMeters, zoneID); err != nil {
			return err
		}
	}
	if req.MinJoinMinutes != nil {
		if _, err := runner.ExecContext(ctx, `UPDATE office_zones SET min_join_minutes = $1, updated_at = NOW() WHERE id = $2`, *req.MinJoinMinutes, zoneID); err != nil {
			return err
		}
	}
	if req.MinCancelMinutes != nil {
		if _, err := runner.ExecContext(ctx, `UPDATE office_zones SET min_cancel_minutes = $1, updated_at = NOW() WHERE id = $2`, *req.MinCancelMinutes, zoneID); err != nil {
			return err
		}
	}
	if req.MaxSeats != nil {
		if _, err := runner.ExecContext(ctx, `UPDATE office_zones SET max_seats = $1, updated_at = NOW() WHERE id = $2`, *req.MaxSeats, zoneID); err != nil {
			return err
		}
	}
	return nil
}

func (r *OfficeRepo) FindZoneContainingPoint(ctx context.Context, officeID uuid.UUID, lat, lng float64) (*model.OfficeZone, error) {
	var zone struct {
		ID                uuid.UUID `db:"id"`
		OfficeID          uuid.UUID `db:"office_id"`
		PickupZoneGeoJSON string    `db:"pickup_zone_geojson"`
		MaxDetourMinutes  int       `db:"max_detour_minutes"`
		MaxDistanceMeters int       `db:"max_distance_meters"`
		MinJoinMinutes    int       `db:"min_join_minutes"`
		MinCancelMinutes  int       `db:"min_cancel_minutes"`
		MaxSeats          int       `db:"max_seats"`
		IsActive          bool      `db:"is_active"`
	}
	err := getDbRunner(ctx, r.db).GetContext(ctx, &zone, `
		SELECT id, office_id,
		       ST_AsGeoJSON(pickup_zone) AS pickup_zone_geojson,
		       max_detour_minutes, max_distance_meters,
		       min_join_minutes, min_cancel_minutes, max_seats, is_active
		FROM office_zones
		WHERE office_id = $1
		  AND is_active = true
		  AND ST_Within(
		      ST_SetSRID(ST_MakePoint($3, $2), 4326),
		      pickup_zone
		  )
		LIMIT 1
	`, officeID, lat, lng)
	if err == sql.ErrNoRows {
		return nil, model.ErrOriginOutsideZone
	}
	if err != nil {
		return nil, err
	}
	return &model.OfficeZone{
		ID:                zone.ID,
		OfficeID:          zone.OfficeID,
		PickupZoneGeoJSON: zone.PickupZoneGeoJSON,
		MaxDetourMinutes:  zone.MaxDetourMinutes,
		MaxDistanceMeters: zone.MaxDistanceMeters,
		MinJoinMinutes:    zone.MinJoinMinutes,
		MinCancelMinutes:  zone.MinCancelMinutes,
		MaxSeats:          zone.MaxSeats,
		IsActive:          zone.IsActive,
	}, nil
}
