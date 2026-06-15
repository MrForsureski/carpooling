-- migrations/005_create_trips.sql
-- +goose Up

CREATE TYPE trip_status AS ENUM (
    'scheduled',
    'in_progress',
    'completed',
    'cancelled'
);

CREATE TABLE trips (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    driver_id        UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    office_id        UUID NOT NULL REFERENCES offices(id) ON DELETE RESTRICT,
    zone_id          UUID REFERENCES office_zones(id) ON DELETE SET NULL,
    origin           GEOMETRY(Point, 4326) NOT NULL,
    origin_address   TEXT,
    depart_at        TIMESTAMPTZ NOT NULL,
    seats_total      INTEGER NOT NULL CHECK (seats_total BETWEEN 1 AND 20),
    seats_left       INTEGER NOT NULL CHECK (seats_left >= 0),
    route_geojson    TEXT NOT NULL,
    duration_seconds INTEGER NOT NULL,
    distance_meters  INTEGER NOT NULL,
    status           trip_status NOT NULL DEFAULT 'scheduled',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT seats_left_lte_total CHECK (seats_left <= seats_total)
);

CREATE INDEX idx_trips_driver_id ON trips(driver_id);
CREATE INDEX idx_trips_office_id ON trips(office_id);
CREATE INDEX idx_trips_depart_at ON trips(depart_at);
CREATE INDEX idx_trips_status ON trips(status);
CREATE INDEX idx_trips_origin ON trips USING GIST(origin);

-- +goose Down
DROP TABLE trips;
DROP TYPE trip_status;
