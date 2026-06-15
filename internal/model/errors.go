package model

import (
	"errors"
	"fmt"
)

// Sentinel ошибки
var (
	ErrTripNotFound          = errors.New("trip not found")
	ErrTripNotScheduled      = errors.New("trip is not in scheduled status")
	ErrNoSeatsAvailable      = errors.New("no seats available")
	ErrCannotJoinOwnTrip     = errors.New("driver cannot join own trip")
	ErrJoinTooLate           = errors.New("too late to join trip")
	ErrCancelTooLate         = errors.New("too late to cancel trip participation")
	ErrAlreadyInTrip         = errors.New("already in this trip")
	ErrSchedulingConflict    = errors.New("scheduling conflict with another trip")
	ErrOriginOutsideZone     = errors.New("origin point is outside the allowed zone")
	ErrTooSoonToCreate       = errors.New("trip must be created at least 60 minutes before departure")
	ErrUserNotFound          = errors.New("user not found")
	ErrCompanyNotFound       = errors.New("company not found")
	ErrOfficeNotFound        = errors.New("office not found")
	ErrZoneNotFound          = errors.New("zone not found")
	ErrEmailDomainNotAllowed = errors.New("email domain not allowed for this company")
	ErrStopNotFound          = errors.New("stop not found")
	ErrInvalidCredentials    = errors.New("invalid email or password")
	ErrEmailNotVerified      = errors.New("email not verified")
	ErrEmailAlreadyExists    = errors.New("email already registered")
	ErrInvalidToken          = errors.New("invalid or expired token")
	ErrForbidden             = errors.New("access forbidden")
	ErrDriverConflict        = errors.New("driver already has an active trip at this time")
	ErrTripAlreadyPassed     = errors.New("cannot cancel a trip that has already started or passed")
)

// ErrTooFarFromRoute — пассажир слишком далеко от маршрута
type ErrTooFarFromRoute struct {
	ActualMeters int
	MaxMeters    int
}

func (e *ErrTooFarFromRoute) Error() string {
	return fmt.Sprintf("pickup point is %.1f km from route, maximum is %.1f km",
		float64(e.ActualMeters)/1000,
		float64(e.MaxMeters)/1000,
	)
}

// ErrDetourExceeded — крюк для водителя превышает лимит
type ErrDetourExceeded struct {
	ActualMinutes int
	MaxMinutes    int
}

func (e *ErrDetourExceeded) Error() string {
	return fmt.Sprintf("detour for driver would be %d min, maximum is %d min",
		e.ActualMinutes, e.MaxMinutes,
	)
}
