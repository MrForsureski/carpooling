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

// Createtrip создание новой поездки
func (s *TripService) CreateTrip(ctx context.Context, driverID uuid.UUID, req dto.CreateTripRequest) (*model.Trip, error) {
	officeID, err := uuid.Parse(req.OfficeID)
	if err != nil {
		return nil, fmt.Errorf("invalid office_id: %w", err)
	}

	departAt, err := time.Parse(time.RFC3339, req.DepartAt)
	if err != nil {
		return nil, fmt.Errorf("invalid depart_at format (use RFC3339): %w", err)
	}

	// Проверка минимального времени до старта 60 минут
	if time.Until(departAt) < 60*time.Minute {
		return nil, model.ErrTooSoonToCreate
	}

	//проверка нахождения точки старта в зоне офиса
	zone, err := s.officeRepo.FindZoneContainingPoint(ctx, officeID, req.OriginLat, req.OriginLng)
	if err != nil {
		return nil, model.ErrOriginOutsideZone
	}

	// Проверка отсутствия конфликта по расписанию у водителя
	conflict, err := s.tripRepo.HasDriverConflict(ctx, driverID, departAt)
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, model.ErrDriverConflict
	}

	//Получение офиса для координат назначения
	office, err := s.officeRepo.GetByID(ctx, officeID)
	if err != nil {
		return nil, err
	}

	//запрос маршрута у osrm через все остановки
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

	//Коэффициент замедления для реального маршрута
	//Osrm расчет для легкового авто умноженный на коэффициент
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

	//создание остановок
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

// Jointrip присоединение пассажира к поездке
func (s *TripService) JoinTrip(ctx context.Context, passengerID uuid.UUID, tripID uuid.UUID, req dto.JoinTripRequest) (*model.TripPassenger, error) {
	var passenger *model.TripPassenger
	err := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		// Получение поездки с block for update в транзакции
		trip, err := s.tripRepo.GetForUpdate(txCtx, tripID)
		if err != nil {
			return model.ErrTripNotFound
		}

		// Загрузка зоны
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

		// Проверка нахождения в поездке
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

		//Использование доменной модели
		if err := trip.CanJoin(passengerID); err != nil {
			return err
		}

		//Проверка конфликта расписания у пассажира
		conflict, _ := s.tripRepo.HasPassengerConflict(txCtx, passengerID, trip.DepartAt)
		if conflict {
			return model.ErrSchedulingConflict
		}

		//Получение остановки
		stopID, err := uuid.Parse(req.StopID)
		if err != nil {
			return fmt.Errorf("invalid stop_id: %w", err)
		}

		stop, err := s.tripRepo.GetStopByID(txCtx, stopID)
		if err != nil {
			return model.ErrStopNotFound
		}

		//создание записи
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

//leavetrip выход пассажира из поездки
func (s *TripService) LeaveTrip(ctx context.Context, passengerID uuid.UUID, tripID uuid.UUID) error {
	trip, err := s.tripRepo.GetByID(ctx, tripID)
	if err != nil {
		return model.ErrTripNotFound
	}

	if trip.Status != model.TripStatusScheduled {
		return model.ErrTripNotScheduled
	}

	// загрузка зоны
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

//Canceltrip отмена поездки водителем или админом
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

//Gettrip получение поездки по id
func (s *TripService) GetTrip(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	return s.tripRepo.GetByID(ctx, id)
}

//searchtrips поиск поездок по офису дате и точке пассажира
func (s *TripService) SearchTrips(ctx context.Context, officeID uuid.UUID, date time.Time, lat, lng float64, limit, offset int) ([]*model.Trip, int, error) {
	//Получение максимального расстояния из зоны офиса
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

// Listmytrips поездки водителя
func (s *TripService) ListMyTrips(ctx context.Context, driverID uuid.UUID, limit, offset int) ([]*model.Trip, int, error) {
	return s.tripRepo.ListByDriver(ctx, driverID, limit, offset)
}

// Listjoinedtrips поездки пассажира
func (s *TripService) ListJoinedTrips(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*model.Trip, int, error) {
	return s.tripRepo.ListJoined(ctx, userID, limit, offset)
}

//getroutepreview превью маршрута
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

//getpassengers получение пассажиров поездки
func (s *TripService) GetPassengers(ctx context.Context, tripID uuid.UUID) ([]*model.TripPassenger, error) {
	return s.tripRepo.GetPassengers(ctx, tripID)
}

// getstopsbytripid получение остановок поездки
func (s *TripService) GetStopsByTripID(ctx context.Context, tripID uuid.UUID) ([]*model.TripStop, error) {
	return s.tripRepo.GetStopsByTripID(ctx, tripID)
}

// Listalltrips поездки для админки
func (s *TripService) ListAllTrips(ctx context.Context, limit, offset int) ([]*model.Trip, int, error) {
	return s.tripRepo.ListAll(ctx, limit, offset)
}
