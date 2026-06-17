package model

import (
	"time"

	"github.com/google/uuid"
)

type Office struct {
	ID        uuid.UUID `db:"id"         json:"id"`
	CompanyID uuid.UUID `db:"company_id" json:"company_id"`
	Name      string    `db:"name"       json:"name"`
	Address   string    `db:"address"    json:"address"`
	//wkt геометрия из бд
	LocationWKT string  `db:"location"   json:"-"`
	Lat         float64 `db:"-"          json:"lat"`
	Lng         float64 `db:"-"          json:"lng"`
	IsActive    bool    `db:"is_active"  json:"is_active"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`

	//Joined поля
	ZoneGeoJSON string `db:"-" json:"zone_geojson,omitempty"`
}

type OfficeZone struct {
	ID                 uuid.UUID `db:"id"                   json:"id"`
	OfficeID           uuid.UUID `db:"office_id"            json:"office_id"`
	PickupZoneGeoJSON  string    `db:"pickup_zone_geojson"  json:"pickup_zone_geojson"`
	MaxDetourMinutes   int       `db:"max_detour_minutes"   json:"max_detour_minutes"`
	MaxDistanceMeters  int       `db:"max_distance_meters"  json:"max_distance_meters"`
	MinJoinMinutes     int       `db:"min_join_minutes"     json:"min_join_minutes"`
	MinCancelMinutes   int       `db:"min_cancel_minutes"   json:"min_cancel_minutes"`
	MaxSeats           int       `db:"max_seats"            json:"max_seats"`
	IsActive           bool      `db:"is_active"            json:"is_active"`
	CreatedAt          time.Time `db:"created_at"           json:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"           json:"updated_at"`
}

