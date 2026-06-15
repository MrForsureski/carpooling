package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTrip_CanJoin(t *testing.T) {
	driverID := uuid.New()
	passengerID := uuid.New()
	officeID := uuid.New()

	tests := []struct {
		name    string
		trip    Trip
		passID  uuid.UUID
		wantErr error
	}{
		{
			name: "Success: Scheduled trip with seats left, driver is different, departs in future",
			trip: Trip{
				Status:    TripStatusScheduled,
				SeatsLeft: 3,
				DriverID:  driverID,
				DepartAt:  time.Now().Add(2 * time.Hour),
			},
			passID:  passengerID,
			wantErr: nil,
		},
		{
			name: "Success: Scheduled trip using Zone constraints",
			trip: Trip{
				Status:    TripStatusScheduled,
				SeatsLeft: 2,
				DriverID:  driverID,
				DepartAt:  time.Now().Add(45 * time.Minute),
				Zone: &OfficeZone{
					MinJoinMinutes: 30,
				},
			},
			passID:  passengerID,
			wantErr: nil,
		},
		{
			name: "Error: Trip is completed",
			trip: Trip{
				Status:    TripStatusCompleted,
				SeatsLeft: 3,
				DriverID:  driverID,
				DepartAt:  time.Now().Add(2 * time.Hour),
			},
			passID:  passengerID,
			wantErr: ErrTripNotScheduled,
		},
		{
			name: "Error: No seats left",
			trip: Trip{
				Status:    TripStatusScheduled,
				SeatsLeft: 0,
				DriverID:  driverID,
				DepartAt:  time.Now().Add(2 * time.Hour),
			},
			passID:  passengerID,
			wantErr: ErrNoSeatsAvailable,
		},
		{
			name: "Error: Passenger is driver",
			trip: Trip{
				Status:    TripStatusScheduled,
				SeatsLeft: 3,
				DriverID:  driverID,
				DepartAt:  time.Now().Add(2 * time.Hour),
			},
			passID:  driverID,
			wantErr: ErrCannotJoinOwnTrip,
		},
		{
			name: "Error: Too late to join (min join minutes default is 30, departing in 15)",
			trip: Trip{
				Status:    TripStatusScheduled,
				SeatsLeft: 3,
				DriverID:  driverID,
				DepartAt:  time.Now().Add(15 * time.Minute),
			},
			passID:  passengerID,
			wantErr: ErrJoinTooLate,
		},
		{
			name: "Error: Too late to join with custom zone constraints (min join minutes is 60, departing in 45)",
			trip: Trip{
				Status:    TripStatusScheduled,
				SeatsLeft: 3,
				DriverID:  driverID,
				DepartAt:  time.Now().Add(45 * time.Minute),
				Zone: &OfficeZone{
					ID:             uuid.New(),
					OfficeID:       officeID,
					MinJoinMinutes: 60,
				},
			},
			passID:  passengerID,
			wantErr: ErrJoinTooLate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.trip.CanJoin(tt.passID)
			if err != tt.wantErr {
				t.Errorf("Trip.CanJoin() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
