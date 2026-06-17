package osrm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"office_trip/internal/service"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// getroute построение маршрута через несколько точек
// points координаты lat lng минимум 2 точки старт и финиш
func (c *Client) GetRoute(ctx context.Context, points ...float64) (*service.RouteResult, error) {
	if len(points)%2 != 0 || len(points) < 4 {
		return nil, fmt.Errorf("invalid points: need pairs of lat,lng, got %d values", len(points))
	}

	// osrm формат координат lng lat
	coords := make([]string, 0, len(points)/2)
	for i := 0; i < len(points); i += 2 {
		lat, lng := points[i], points[i+1]
		coords = append(coords, fmt.Sprintf("%.6f,%.6f", lng, lat))
	}

	url := fmt.Sprintf("%s/route/v1/driving/%s?overview=full&geometries=geojson",
		c.baseURL,
		strings.Join(coords, ";"),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "OfficeTripApp/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OSRM request failed: %w", err)
	}
	defer resp.Body.Close()

	var osrmResp struct {
		Code   string `json:"code"`
		Routes []struct {
			Duration float64 `json:"duration"`
			Distance float64 `json:"distance"`
			Geometry struct {
				Type        string      `json:"type"`
				Coordinates [][]float64 `json:"coordinates"`
			} `json:"geometry"`
			Legs []struct {
				Duration float64 `json:"duration"`
				Distance float64 `json:"distance"`
			} `json:"legs"`
		} `json:"routes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&osrmResp); err != nil {
		return nil, fmt.Errorf("OSRM response decode error: %w", err)
	}

	if osrmResp.Code != "Ok" || len(osrmResp.Routes) == 0 {
		return nil, fmt.Errorf("OSRM returned no routes, code: %s", osrmResp.Code)
	}

	route := osrmResp.Routes[0]
	geojsonBytes, err := json.Marshal(route.Geometry)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal geometry: %w", err)
	}

	legDurations := make([]int, len(route.Legs))
	for i, leg := range route.Legs {
		legDurations[i] = int(leg.Duration)
	}

	return &service.RouteResult{
		GeoJSON:         string(geojsonBytes),
		DurationSeconds: int(route.Duration),
		DistanceMeters:  int(route.Distance),
		LegDurations:    legDurations,
	}, nil
}
