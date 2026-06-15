package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"office_trip/internal/dto"
	"office_trip/internal/model"
	"office_trip/internal/repository"
)

type TripService struct {
	tripRepo   repository.TripRepository
	officeRepo repository.OfficeRepository
	userRepo   repository.UserRepository
	routing    RoutingProvider
	txManager  TransactionManager
}

func NewTripService(
	tripRepo repository.TripRepository,
	officeRepo repository.OfficeRepository,
	userRepo repository.UserRepository,
	routing RoutingProvider,
	txManager TransactionManager,
) *TripService {
	return &TripService{
		tripRepo:   tripRepo,
		officeRepo: officeRepo,
		userRepo:   userRepo,
		routing:    routing,
		txManager:  txManager,
	}
}

// CreateTrip создаёт новую поездку
func (s *TripService) CreateTrip(ctx context.Context, driverID uuid.UUID, req dto.CreateTripRequest) (*model.Trip, error) {
	officeID, err := uuid.Parse(req.OfficeID)
	if err != nil {
		return nil, fmt.Errorf("invalid office_id: %w", err)
	}

	departAt, err := time.Parse(time.RFC3339, req.DepartAt)
	if err != nil {
		return nil, fmt.Errorf("invalid depart_at format (use RFC3339): %w", err)
	}

	// 1. Проверяем минимальное время до старта (60 мин)
	if time.Until(departAt) < 60*time.Minute {
		return nil, model.ErrTooSoonToCreate
	}

	// 2. Проверяем, что точка старта в зоне офиса
	zone, err := s.officeRepo.FindZoneContainingPoint(ctx, officeID, req.OriginLat, req.OriginLng)
	if err != nil {
		return nil, model.ErrOriginOutsideZone
	}

	// 3. Проверяем, нет ли конфликта по расписанию у водителя
	conflict, err := s.tripRepo.HasDriverConflict(ctx, driverID, departAt)
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, model.ErrDriverConflict
	}

	// 4. Получаем офис для координат назначения
	office, err := s.officeRepo.GetByID(ctx, officeID)
	if err != nil {
		return nil, err
	}

	// 5. Запрашиваем маршрут у OSRM через все остановки
	points := make([]float64, 0, 4+len(req.Stops)*2)
	points = append(points, req.OriginLat, req.OriginLng)
	for _, stop := range req.Stops {
		points = append(points, stop.Lat, stop.Lng)
	}
	points = append(points, office.Lat, office.Lng)

	route, err := s.routing.GetRoute(ctx, points...)
	if err != nil {
		return nil, fmt.Errorf("routing service error: %w", err)
	}

	// Коэффициент замедления для реального маршрута (пробки, остановки, автобусный режим)
	// OSRM рассчитывает для легкового авто на пустой дороге, ×1.4 даёт реалистичное время
	const busSlowdownFactor = 1.4
	adjustedDuration := int(float64(route.DurationSeconds) * busSlowdownFactor)
	adjustedLegDurations := make([]int, len(route.LegDurations))
	for i, d := range route.LegDurations {
		adjustedLegDurations[i] = int(float64(d) * busSlowdownFactor)
	}

	zoneID := zone.ID
	trip := &model.Trip{
		ID:              uuid.New(),
		DriverID:        driverID,
		OfficeID:        officeID,
		ZoneID:          &zoneID,
		OriginLat:       req.OriginLat,
		OriginLng:       req.OriginLng,
		OriginAddress:   req.OriginAddress,
		DepartAt:        departAt,
		SeatsTotal:      req.SeatsTotal,
		SeatsLeft:       req.SeatsTotal,
		RouteGeoJSON:    route.GeoJSON,
		DurationSeconds: adjustedDuration,
		DistanceMeters:  route.DistanceMeters,
		Status:          model.TripStatusScheduled,
	}

	if err := s.tripRepo.Create(ctx, trip); err != nil {
		return nil, err
	}

	// 6. Создаем остановки
	accumulatedDuration := 0
	tripStops := make([]*model.TripStop, len(req.Stops))
	for i, rStop := range req.Stops {
		if i < len(adjustedLegDurations) {
			accumulatedDuration += adjustedLegDurations[i]
		}
		arrivalTime := departAt.Add(time.Duration(accumulatedDuration) * time.Second)
		stop := &model.TripStop{
			ID:             uuid.New(),
			TripID:         trip.ID,
			Lat:            rStop.Lat,
			Lng:            rStop.Lng,
			Address:        rStop.Address,
			ArrivalTime:    arrivalTime,
			SequenceNumber: i + 1,
		}
		if err := s.tripRepo.CreateStop(ctx, stop); err != nil {
			return nil, fmt.Errorf("failed to create stop: %w", err)
		}
		tripStops[i] = stop
	}
	trip.Stops = tripStops

	return trip, nil
}

// JoinTrip — присоединение пассажира к поездке
func (s *TripService) JoinTrip(ctx context.Context, passengerID uuid.UUID, tripID uuid.UUID, req dto.JoinTripRequest) (*model.TripPassenger, error) {
	var passenger *model.TripPassenger
	err := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		// Получаем поездку (с FOR UPDATE т.к. мы в транзакции!)
		trip, err := s.tripRepo.GetForUpdate(txCtx, tripID)
		if err != nil {
			return model.ErrTripNotFound
		}

		// Загружаем зону ограничений
		var zone *model.OfficeZone
		if trip.ZoneID != nil {
			zone, _ = s.officeRepo.GetZoneByOffice(txCtx, trip.OfficeID)
		}
		if zone == nil {
			// Дефолтные значения
			zone = &model.OfficeZone{
				MaxDetourMinutes:  15,
				MaxDistanceMeters: 2000,
				MinJoinMinutes:    30,
				MinCancelMinutes:  30,
			}
		}
		trip.Zone = zone

		// Проверка — уже в поездке?
		alreadyIn, _ := s.tripRepo.IsPassengerInTrip(txCtx, tripID, passengerID)
		if alreadyIn {
			stopID, err := uuid.Parse(req.StopID)
			if err != nil {
				return fmt.Errorf("invalid stop_id: %w", err)
			}
			stop, err := s.tripRepo.GetStopByID(txCtx, stopID)
			if err != nil {
				return model.ErrStopNotFound
			}

			err = s.tripRepo.UpdatePassengerStop(txCtx, tripID, passengerID, stop.ID, stop.Address, stop.Lat, stop.Lng)
			if err != nil {
				return err
			}

			passenger = &model.TripPassenger{
				TripID:        tripID,
				UserID:        passengerID,
				PickupAddress: stop.Address,
				PickupLat:     stop.Lat,
				PickupLng:     stop.Lng,
				StopID:        &stop.ID,
			}
			return nil
		}

		// --- Использование богатой доменной модели ---
		if err := trip.CanJoin(passengerID); err != nil {
			return err
		}

		// Проверка конфликта расписания у пассажира
		conflict, _ := s.tripRepo.HasPassengerConflict(txCtx, passengerID, trip.DepartAt)
		if conflict {
			return model.ErrSchedulingConflict
		}

		// Получаем остановку
		stopID, err := uuid.Parse(req.StopID)
		if err != nil {
			return fmt.Errorf("invalid stop_id: %w", err)
		}

		stop, err := s.tripRepo.GetStopByID(txCtx, stopID)
		if err != nil {
			return model.ErrStopNotFound
		}

		// --- Всё проверено, создаём запись ---
		passenger = &model.TripPassenger{
			ID:            uuid.New(),
			TripID:        tripID,
			UserID:        passengerID,
			PickupLat:     stop.Lat,
			PickupLng:     stop.Lng,
			PickupAddress: stop.Address,
			DetourSeconds: 0,
			Status:        "confirmed",
			StopID:        &stop.ID,
		}

		if err := s.tripRepo.AddPassenger(txCtx, passenger); err != nil {
			return err
		}

		if err := s.tripRepo.DecrementSeats(txCtx, tripID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return passenger, nil
}

// LeaveTrip — пассажир покидает поездку
func (s *TripService) LeaveTrip(ctx context.Context, passengerID uuid.UUID, tripID uuid.UUID) error {
	trip, err := s.tripRepo.GetByID(ctx, tripID)
	if err != nil {
		return model.ErrTripNotFound
	}

	if trip.Status != model.TripStatusScheduled {
		return model.ErrTripNotScheduled
	}

	// Загружаем зону для ограничений
	zone, _ := s.officeRepo.GetZoneByOffice(ctx, trip.OfficeID)
	minCancel := 30
	if zone != nil {
		minCancel = zone.MinCancelMinutes
	}

	if time.Until(trip.DepartAt) < time.Duration(minCancel)*time.Minute {
		return model.ErrCancelTooLate
	}

	if err := s.tripRepo.CancelPassenger(ctx, tripID, passengerID); err != nil {
		return err
	}

	if err := s.tripRepo.IncrementSeats(ctx, tripID); err != nil {
		return err
	}



	return nil
}

// CancelTrip — водитель или admin отменяет поездку
func (s *TripService) CancelTrip(ctx context.Context, userID uuid.UUID, tripID uuid.UUID, isAdmin bool) error {
	trip, err := s.tripRepo.GetByID(ctx, tripID)
	if err != nil {
		return model.ErrTripNotFound
	}

	if !isAdmin && trip.DriverID != userID {
		return model.ErrForbidden
	}

	if trip.Status == model.TripStatusCancelled || trip.Status == model.TripStatusCompleted {
		return model.ErrTripNotScheduled
	}

	if time.Now().After(trip.DepartAt) {
		return model.ErrTripAlreadyPassed
	}

	if err := s.tripRepo.UpdateStatus(ctx, tripID, model.TripStatusCancelled); err != nil {
		return err
	}



	return nil
}

// GetTrip возвращает поездку по ID
func (s *TripService) GetTrip(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	return s.tripRepo.GetByID(ctx, id)
}

// SearchTrips — поиск поездок по офису, дате и точке пассажира
func (s *TripService) SearchTrips(ctx context.Context, officeID uuid.UUID, date time.Time, lat, lng float64, limit, offset int) ([]*model.Trip, int, error) {
	// Получаем максимальное расстояние из зоны офиса
	maxDist := 2000.0
	zone, err := s.officeRepo.GetZoneByOffice(ctx, officeID)
	if err == nil && zone != nil {
		maxDist = float64(zone.MaxDistanceMeters)
	}

	trips, total, err := s.tripRepo.List(ctx, officeID, date, lat, lng, maxDist, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	for _, t := range trips {
		stops, _ := s.tripRepo.GetStopsByTripID(ctx, t.ID)
		t.Stops = stops
	}

	return trips, total, nil
}

// ListMyTrips — поездки водителя
func (s *TripService) ListMyTrips(ctx context.Context, driverID uuid.UUID, limit, offset int) ([]*model.Trip, int, error) {
	return s.tripRepo.ListByDriver(ctx, driverID, limit, offset)
}

// ListJoinedTrips — поездки, в которых пользователь пассажир
func (s *TripService) ListJoinedTrips(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*model.Trip, int, error) {
	return s.tripRepo.ListJoined(ctx, userID, limit, offset)
}

// GetRoutePreview — превью маршрута без создания поездки
func (s *TripService) GetRoutePreview(ctx context.Context, originLat, originLng float64, officeID uuid.UUID, stopCoords ...float64) (*RouteResult, error) {
	office, err := s.officeRepo.GetByID(ctx, officeID)
	if err != nil {
		return nil, err
	}
	points := make([]float64, 0, 4 + len(stopCoords))
	points = append(points, originLat, originLng)
	points = append(points, stopCoords...)
	points = append(points, office.Lat, office.Lng)
	return s.routing.GetRoute(ctx, points...)
}

// GetPassengers возвращает пассажиров поездки
func (s *TripService) GetPassengers(ctx context.Context, tripID uuid.UUID) ([]*model.TripPassenger, error) {
	return s.tripRepo.GetPassengers(ctx, tripID)
}

// GetStopsByTripID возвращает остановки поездки
func (s *TripService) GetStopsByTripID(ctx context.Context, tripID uuid.UUID) ([]*model.TripStop, error) {
	return s.tripRepo.GetStopsByTripID(ctx, tripID)
}

// ListAllTrips — для admin панели
func (s *TripService) ListAllTrips(ctx context.Context, limit, offset int) ([]*model.Trip, int, error) {
	return s.tripRepo.ListAll(ctx, limit, offset)
}
