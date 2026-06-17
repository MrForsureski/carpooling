package dto

// Createofficerequest запрос на создание офиса
type CreateOfficeRequest struct {
	Name    string  `json:"name"    validate:"required,min=2,max=255"`
	Address string  `json:"address" validate:"required,min=5,max=500"`
	Lat     float64 `json:"lat"     validate:"required,latitude"`
	Lng     float64 `json:"lng"     validate:"required,longitude"`
}

// createzonerequest запрос на создание зоны офиса
type CreateZoneRequest struct {
	PickupZone         map[string]interface{} `json:"pickup_zone"          validate:"required"`
	MaxDetourMinutes   int                    `json:"max_detour_minutes"   validate:"min=1,max=60"`
	MaxDistanceMeters  int                    `json:"max_distance_meters"  validate:"min=100,max=10000"`
	MinJoinMinutes     int                    `json:"min_join_minutes"     validate:"min=5,max=180"`
	MinCancelMinutes   int                    `json:"min_cancel_minutes"   validate:"min=5,max=180"`
	MaxSeats           int                    `json:"max_seats"            validate:"min=1,max=20"`
}

// updatezonerequest обновление ограничений зоны
type UpdateZoneRequest struct {
	MaxDetourMinutes  *int `json:"max_detour_minutes"   validate:"omitempty,min=1,max=60"`
	MaxDistanceMeters *int `json:"max_distance_meters"  validate:"omitempty,min=100,max=10000"`
	MinJoinMinutes    *int `json:"min_join_minutes"     validate:"omitempty,min=5,max=180"`
	MinCancelMinutes  *int `json:"min_cancel_minutes"   validate:"omitempty,min=5,max=180"`
	MaxSeats          *int `json:"max_seats"            validate:"omitempty,min=1,max=20"`
}
