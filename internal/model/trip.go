package model

import (
	"time"

	"github.com/google/uuid"
)

type TripStatus string

const (
	TripStatusScheduled  TripStatus = "scheduled"
	TripStatusInProgress TripStatus = "in_progress"
	TripStatusCompleted  TripStatus = "completed"
	TripStatusCancelled  TripStatus = "cancelled"
)

type Trip struct {
	ID              uuid.UUID  `db:"id"               json:"id"`
	DriverID        uuid.UUID  `db:"driver_id"        json:"driver_id"`
	OfficeID        uuid.UUID  `db:"office_id"        json:"office_id"`
	ZoneID          *uuid.UUID `db:"zone_id"          json:"zone_id,omitempty"`
	OriginWKT       string     `db:"origin"           json:"-"`
	OriginLat       float64    `db:"-"                json:"origin_lat"`
	OriginLng       float64    `db:"-"                json:"origin_lng"`
	OriginAddress   string     `db:"origin_address"   json:"origin_address"`
	DepartAt        time.Time  `db:"depart_at"        json:"depart_at"`
	SeatsTotal      int        `db:"seats_total"      json:"seats_total"`
	SeatsLeft       int        `db:"seats_left"       json:"seats_left"`
	RouteGeoJSON    string     `db:"route_geojson"    json:"route_geojson"`
	DurationSeconds int        `db:"duration_seconds" json:"duration_seconds"`
	DistanceMeters  int        `db:"distance_meters"  json:"distance_meters"`
	Status          TripStatus `db:"status"           json:"status"`
	CreatedAt       time.Time  `db:"created_at"       json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"       json:"updated_at"`

	// Joined поля
	DriverName      string     `db:"driver_name"       json:"driver_name,omitempty"`
	OfficeName      string     `db:"office_name"       json:"office_name,omitempty"`
	DistanceToRoute float64    `db:"distance_to_route" json:"distance_to_route,omitempty"`

	// Runtime (для бизнес-логики, не из БД)
	Zone *OfficeZone `db:"-" json:"-"`
	Stops []*TripStop `db:"-" json:"stops,omitempty"`
	UserPickupStop *TripStop `db:"-" json:"user_pickup_stop,omitempty"`
}

// CanJoin проверяет инварианты возможности присоединения пассажира к поездке.
func (t *Trip) CanJoin(passengerID uuid.UUID) error {
	if t.Status != TripStatusScheduled {
		return ErrTripNotScheduled
	}
	if t.SeatsLeft <= 0 {
		return ErrNoSeatsAvailable
	}
	if t.DriverID == passengerID {
		return ErrCannotJoinOwnTrip
	}
	minJoinMinutes := 30
	if t.Zone != nil {
		minJoinMinutes = t.Zone.MinJoinMinutes
	}
	if time.Until(t.DepartAt) < time.Duration(minJoinMinutes)*time.Minute {
		return ErrJoinTooLate
	}
	return nil
}

type TripStop struct {
	ID             uuid.UUID `db:"id"              json:"id"`
	TripID         uuid.UUID `db:"trip_id"         json:"trip_id"`
	Lat            float64   `db:"lat"             json:"lat"`
	Lng            float64   `db:"lng"             json:"lng"`
	Address        string    `db:"address"         json:"address"`
	ArrivalTime    time.Time `db:"arrival_time"    json:"arrival_time"`
	SequenceNumber int       `db:"sequence_number" json:"sequence_number"`
}

type TripPassenger struct {
	ID            uuid.UUID  `db:"id"             json:"id"`
	TripID        uuid.UUID  `db:"trip_id"        json:"trip_id"`
	UserID        uuid.UUID  `db:"user_id"        json:"user_id"`
	PickupWKT     string     `db:"pickup"         json:"-"`
	PickupLat     float64    `db:"-"              json:"pickup_lat"`
	PickupLng     float64    `db:"-"              json:"pickup_lng"`
	PickupAddress string     `db:"pickup_address" json:"pickup_address"`
	DetourSeconds int        `db:"detour_seconds" json:"detour_seconds"`
	Status        string     `db:"status"         json:"status"`
	JoinedAt      time.Time  `db:"joined_at"      json:"joined_at"`
	CancelledAt   *time.Time `db:"cancelled_at"   json:"cancelled_at,omitempty"`

	// Joined
	UserFullName string `db:"user_full_name" json:"user_full_name,omitempty"`
	StopID       *uuid.UUID `db:"stop_id" json:"stop_id,omitempty"`
}

