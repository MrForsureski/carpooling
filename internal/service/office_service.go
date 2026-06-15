package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"office_trip/internal/dto"
	"office_trip/internal/model"
	"office_trip/internal/repository"
)

type OfficeService struct {
	officeRepo repository.OfficeRepository
}

func NewOfficeService(officeRepo repository.OfficeRepository) *OfficeService {
	return &OfficeService{officeRepo: officeRepo}
}

func (s *OfficeService) CreateOffice(ctx context.Context, companyID uuid.UUID, req dto.CreateOfficeRequest) (*model.Office, error) {
	office := &model.Office{
		ID:        uuid.New(),
		CompanyID: companyID,
		Name:      req.Name,
		Address:   req.Address,
		Lat:       req.Lat,
		Lng:       req.Lng,
		IsActive:  true,
	}
	if err := s.officeRepo.Create(ctx, office); err != nil {
		return nil, err
	}
	return office, nil
}

func (s *OfficeService) GetOffice(ctx context.Context, id uuid.UUID) (*model.Office, error) {
	return s.officeRepo.GetByID(ctx, id)
}

func (s *OfficeService) ListOffices(ctx context.Context, companyID uuid.UUID) ([]*model.Office, error) {
	return s.officeRepo.List(ctx, companyID)
}

func (s *OfficeService) UpdateOffice(ctx context.Context, id uuid.UUID, req dto.CreateOfficeRequest) (*model.Office, error) {
	office, err := s.officeRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	office.Name = req.Name
	office.Address = req.Address
	office.Lat = req.Lat
	office.Lng = req.Lng
	if err := s.officeRepo.Update(ctx, office); err != nil {
		return nil, err
	}
	return office, nil
}

func (s *OfficeService) DeactivateOffice(ctx context.Context, id uuid.UUID) error {
	return s.officeRepo.Deactivate(ctx, id)
}

func (s *OfficeService) CreateZone(ctx context.Context, officeID uuid.UUID, req dto.CreateZoneRequest) (*model.OfficeZone, error) {
	// Сериализуем GeoJSON полигон
	geoJSONStr, err := marshalGeoJSON(req.PickupZone)
	if err != nil {
		return nil, err
	}

	// Дефолты если не указаны
	if req.MaxDetourMinutes == 0 {
		req.MaxDetourMinutes = 15
	}
	if req.MaxDistanceMeters == 0 {
		req.MaxDistanceMeters = 2000
	}
	if req.MinJoinMinutes == 0 {
		req.MinJoinMinutes = 30
	}
	if req.MinCancelMinutes == 0 {
		req.MinCancelMinutes = 30
	}
	if req.MaxSeats == 0 {
		req.MaxSeats = 6
	}

	zone := &model.OfficeZone{
		ID:                uuid.New(),
		OfficeID:          officeID,
		MaxDetourMinutes:  req.MaxDetourMinutes,
		MaxDistanceMeters: req.MaxDistanceMeters,
		MinJoinMinutes:    req.MinJoinMinutes,
		MinCancelMinutes:  req.MinCancelMinutes,
		MaxSeats:          req.MaxSeats,
		IsActive:          true,
	}

	if err := s.officeRepo.CreateZone(ctx, zone, geoJSONStr); err != nil {
		return nil, err
	}
	zone.PickupZoneGeoJSON = geoJSONStr
	return zone, nil
}

func (s *OfficeService) UpdateZone(ctx context.Context, officeID, zoneID uuid.UUID, req dto.UpdateZoneRequest) error {
	return s.officeRepo.UpdateZone(ctx, zoneID, req)
}

func (s *OfficeService) GetZone(ctx context.Context, officeID uuid.UUID) (*model.OfficeZone, error) {
	return s.officeRepo.GetZoneByOffice(ctx, officeID)
}

// marshalGeoJSON конвертирует map в JSON строку
func marshalGeoJSON(m map[string]interface{}) (string, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("invalid GeoJSON: %w", err)
	}
	return string(b), nil
}
