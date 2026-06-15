package dto

import (
	"office_trip/internal/model"
)

// CreateTripRequest — запрос на создание поездки
type CreateTripRequest struct {
	OfficeID      string        `json:"office_id"      validate:"required,uuid4"`
	OriginLat     float64       `json:"origin_lat"     validate:"required,latitude"`
	OriginLng     float64       `json:"origin_lng"     validate:"required,longitude"`
	OriginAddress string        `json:"origin_address" validate:"required,max=500"`
	DepartAt      string        `json:"depart_at"      validate:"required"`
	SeatsTotal    int           `json:"seats_total"    validate:"required,min=1,max=20"`
	Stops         []StopRequest `json:"stops"          validate:"dive"`
}

type StopRequest struct {
	Lat     float64 `json:"lat"     validate:"required,latitude"`
	Lng     float64 `json:"lng"     validate:"required,longitude"`
	Address string  `json:"address" validate:"required,max=500"`
}

// JoinTripRequest — запрос на присоединение
type JoinTripRequest struct {
	PickupLat     float64 `json:"pickup_lat"     validate:"omitempty,latitude"`
	PickupLng     float64 `json:"pickup_lng"     validate:"omitempty,longitude"`
	PickupAddress string  `json:"pickup_address" validate:"omitempty,max=500"`
	StopID        string  `json:"stop_id"        validate:"required,uuid4"`
}

// TripListResponse — ответ при поиске поездок
type TripListResponse struct {
	Trips []model.Trip `json:"trips"`
	Total int          `json:"total"`
	Page  int          `json:"page"`
}

// WSMessage — WebSocket сообщение
type WSMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}
