package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"office_trip/internal/dto"
	"office_trip/internal/model"
)

// Userrepository интерфейс для работы с пользователями
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByVerifyToken(ctx context.Context, token string) (*model.User, error)
	UpdateRole(ctx context.Context, id uuid.UUID, role model.Role) error
	UpdateEmailVerified(ctx context.Context, id uuid.UUID) error
	UpdateRefreshToken(ctx context.Context, id uuid.UUID, tokenHash *string, at *time.Time) error
	UpdateVerifyToken(ctx context.Context, id uuid.UUID, token string, at time.Time) error
	List(ctx context.Context, limit, offset int) ([]*model.User, int, error)
	GetCompanyByEmailDomain(ctx context.Context, domain string) (*model.Company, error)
}

//Officerepository интерфейс для работы с офисами
type OfficeRepository interface {
	Create(ctx context.Context, office *model.Office) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Office, error)
	List(ctx context.Context, companyID uuid.UUID) ([]*model.Office, error)
	Update(ctx context.Context, office *model.Office) error
	Deactivate(ctx context.Context, id uuid.UUID) error

	CreateZone(ctx context.Context, zone *model.OfficeZone, geoJSON string) error
	GetZoneByOffice(ctx context.Context, officeID uuid.UUID) (*model.OfficeZone, error)
	UpdateZone(ctx context.Context, zoneID uuid.UUID, req dto.UpdateZoneRequest) error
	FindZoneContainingPoint(ctx context.Context, officeID uuid.UUID, lat, lng float64) (*model.OfficeZone, error)
}

//triprepository интерфейс для работы с поездками
type TripRepository interface {
	Create(ctx context.Context, trip *model.Trip) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	List(ctx context.Context, officeID uuid.UUID, date time.Time, lat, lng float64, maxDist float64, limit, offset int) ([]*model.Trip, int, error)
	ListByDriver(ctx context.Context, driverID uuid.UUID, limit, offset int) ([]*model.Trip, int, error)
	ListJoined(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*model.Trip, int, error)
	ListAll(ctx context.Context, limit, offset int) ([]*model.Trip, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.TripStatus) error

	HasDriverConflict(ctx context.Context, driverID uuid.UUID, departAt time.Time) (bool, error)
	HasPassengerConflict(ctx context.Context, passengerID uuid.UUID, departAt time.Time) (bool, error)

	GetDistanceToRoute(ctx context.Context, tripID uuid.UUID, lat, lng float64) (float64, error)
	IsPassengerInTrip(ctx context.Context, tripID, userID uuid.UUID) (bool, error)

	AddPassenger(ctx context.Context, passenger *model.TripPassenger) error
	DecrementSeats(ctx context.Context, tripID uuid.UUID) error
	IncrementSeats(ctx context.Context, tripID uuid.UUID) error
	GetPassenger(ctx context.Context, tripID, userID uuid.UUID) (*model.TripPassenger, error)
	CancelPassenger(ctx context.Context, tripID, userID uuid.UUID) error
	UpdatePassengerStop(ctx context.Context, tripID, userID, stopID uuid.UUID, address string, lat, lng float64) error
	GetPassengers(ctx context.Context, tripID uuid.UUID) ([]*model.TripPassenger, error)
	GetPassengerUserIDs(ctx context.Context, tripID uuid.UUID) ([]uuid.UUID, error)

	CreateStop(ctx context.Context, stop *model.TripStop) error
	GetStopsByTripID(ctx context.Context, tripID uuid.UUID) ([]*model.TripStop, error)
	GetStopByID(ctx context.Context, id uuid.UUID) (*model.TripStop, error)
}
