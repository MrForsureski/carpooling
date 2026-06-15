package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"office_trip/internal/dto"
	"office_trip/internal/model"
	"office_trip/internal/repository"
)

type mockTripRepo struct {
	repository.TripRepository
	GetForUpdateFunc        func(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	IsPassengerInTripFunc   func(ctx context.Context, tripID, userID uuid.UUID) (bool, error)
	GetStopByIDFunc         func(ctx context.Context, id uuid.UUID) (*model.TripStop, error)
	AddPassengerFunc        func(ctx context.Context, passenger *model.TripPassenger) error
	DecrementSeatsFunc      func(ctx context.Context, tripID uuid.UUID) error
	UpdatePassengerStopFunc func(ctx context.Context, tripID, userID, stopID uuid.UUID, address string, lat, lng float64) error
	HasPassengerConflictFunc func(ctx context.Context, passengerID uuid.UUID, departAt time.Time) (bool, error)
}

func (m *mockTripRepo) GetForUpdate(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	return m.GetForUpdateFunc(ctx, id)
}

func (m *mockTripRepo) IsPassengerInTrip(ctx context.Context, tripID, userID uuid.UUID) (bool, error) {
	return m.IsPassengerInTripFunc(ctx, tripID, userID)
}

func (m *mockTripRepo) GetStopByID(ctx context.Context, id uuid.UUID) (*model.TripStop, error) {
	return m.GetStopByIDFunc(ctx, id)
}

func (m *mockTripRepo) AddPassenger(ctx context.Context, passenger *model.TripPassenger) error {
	return m.AddPassengerFunc(ctx, passenger)
}

func (m *mockTripRepo) DecrementSeats(ctx context.Context, tripID uuid.UUID) error {
	return m.DecrementSeatsFunc(ctx, tripID)
}

func (m *mockTripRepo) UpdatePassengerStop(ctx context.Context, tripID, userID, stopID uuid.UUID, address string, lat, lng float64) error {
	return m.UpdatePassengerStopFunc(ctx, tripID, userID, stopID, address, lat, lng)
}

func (m *mockTripRepo) HasPassengerConflict(ctx context.Context, passengerID uuid.UUID, departAt time.Time) (bool, error) {
	if m.HasPassengerConflictFunc != nil {
		return m.HasPassengerConflictFunc(ctx, passengerID, departAt)
	}
	return false, nil
}

type mockOfficeRepo struct {
	repository.OfficeRepository
	GetZoneByOfficeFunc func(ctx context.Context, officeID uuid.UUID) (*model.OfficeZone, error)
}

func (m *mockOfficeRepo) GetZoneByOffice(ctx context.Context, officeID uuid.UUID) (*model.OfficeZone, error) {
	return m.GetZoneByOfficeFunc(ctx, officeID)
}

type mockTxManager struct{}

func (m *mockTxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func TestTripService_JoinTrip(t *testing.T) {
	driverID := uuid.New()
	passengerID := uuid.New()
	tripID := uuid.New()
	stopID := uuid.New()

	tests := []struct {
		name         string
		tripRepo     *mockTripRepo
		officeRepo   *mockOfficeRepo
		req          dto.JoinTripRequest
		wantErr      error
		validateResp func(t *testing.T, resp *model.TripPassenger)
	}{
		{
			name: "Success joining new trip",
			req: dto.JoinTripRequest{
				StopID: stopID.String(),
			},
			tripRepo: &mockTripRepo{
				GetForUpdateFunc: func(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
					return &model.Trip{
						ID:        tripID,
						DriverID:  driverID,
						Status:    model.TripStatusScheduled,
						SeatsLeft: 2,
						DepartAt:  time.Now().Add(2 * time.Hour),
					}, nil
				},
				IsPassengerInTripFunc: func(ctx context.Context, tID, uID uuid.UUID) (bool, error) {
					return false, nil
				},
				GetStopByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.TripStop, error) {
					return &model.TripStop{
						ID:      stopID,
						TripID:  tripID,
						Lat:     55.75,
						Lng:     37.61,
						Address: "Moscow",
					}, nil
				},
				AddPassengerFunc: func(ctx context.Context, passenger *model.TripPassenger) error {
					return nil
				},
				DecrementSeatsFunc: func(ctx context.Context, tID uuid.UUID) error {
					return nil
				},
			},
			officeRepo: &mockOfficeRepo{
				GetZoneByOfficeFunc: func(ctx context.Context, officeID uuid.UUID) (*model.OfficeZone, error) {
					return &model.OfficeZone{
						MinJoinMinutes: 30,
					}, nil
				},
			},
			wantErr: nil,
			validateResp: func(t *testing.T, resp *model.TripPassenger) {
				if resp == nil {
					t.Fatal("expected passenger response, got nil")
				}
				if resp.TripID != tripID || resp.UserID != passengerID || *resp.StopID != stopID {
					t.Errorf("unexpected passenger fields: %+v", resp)
				}
			},
		},
		{
			name: "Already joined updates stop",
			req: dto.JoinTripRequest{
				StopID: stopID.String(),
			},
			tripRepo: &mockTripRepo{
				GetForUpdateFunc: func(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
					return &model.Trip{
						ID:        tripID,
						DriverID:  driverID,
						Status:    model.TripStatusScheduled,
						SeatsLeft: 2,
						DepartAt:  time.Now().Add(2 * time.Hour),
					}, nil
				},
				IsPassengerInTripFunc: func(ctx context.Context, tID, uID uuid.UUID) (bool, error) {
					return true, nil
				},
				GetStopByIDFunc: func(ctx context.Context, id uuid.UUID) (*model.TripStop, error) {
					return &model.TripStop{
						ID:      stopID,
						TripID:  tripID,
						Lat:     55.75,
						Lng:     37.61,
						Address: "Moscow New Stop",
					}, nil
				},
				UpdatePassengerStopFunc: func(ctx context.Context, tID, uID, sID uuid.UUID, address string, lat, lng float64) error {
					if tID != tripID || uID != passengerID || sID != stopID || address != "Moscow New Stop" {
						return errors.New("unexpected update params")
					}
					return nil
				},
			},
			officeRepo: &mockOfficeRepo{
				GetZoneByOfficeFunc: func(ctx context.Context, officeID uuid.UUID) (*model.OfficeZone, error) {
					return &model.OfficeZone{}, nil
				},
			},
			wantErr: nil,
			validateResp: func(t *testing.T, resp *model.TripPassenger) {
				if resp == nil {
					t.Fatal("expected passenger response, got nil")
				}
				if *resp.StopID != stopID || resp.PickupAddress != "Moscow New Stop" {
					t.Errorf("unexpected passenger stop update details: %+v", resp)
				}
			},
		},
		{
			name: "Fails with domain rule violation",
			req: dto.JoinTripRequest{
				StopID: stopID.String(),
			},
			tripRepo: &mockTripRepo{
				GetForUpdateFunc: func(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
					return &model.Trip{
						ID:        tripID,
						DriverID:  driverID,
						Status:    model.TripStatusCompleted, // completed status causes domain validation error
						SeatsLeft: 2,
						DepartAt:  time.Now().Add(2 * time.Hour),
					}, nil
				},
				IsPassengerInTripFunc: func(ctx context.Context, tID, uID uuid.UUID) (bool, error) {
					return false, nil
				},
			},
			officeRepo: &mockOfficeRepo{
				GetZoneByOfficeFunc: func(ctx context.Context, officeID uuid.UUID) (*model.OfficeZone, error) {
					return &model.OfficeZone{}, nil
				},
			},
			wantErr: model.ErrTripNotScheduled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewTripService(tt.tripRepo, tt.officeRepo, nil, nil, &mockTxManager{})
			res, err := svc.JoinTrip(context.Background(), passengerID, tripID, tt.req)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("unexpected error = %v, wantErr = %v", err, tt.wantErr)
			}

			if tt.wantErr == nil && tt.validateResp != nil {
				tt.validateResp(t, res)
			}
		})
	}
}
