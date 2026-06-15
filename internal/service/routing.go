package service

import "context"

type RouteResult struct {
	GeoJSON         string
	DurationSeconds int
	DistanceMeters  int
	LegDurations    []int
}

type RoutingProvider interface {
	GetRoute(ctx context.Context, points ...float64) (*RouteResult, error)
}
