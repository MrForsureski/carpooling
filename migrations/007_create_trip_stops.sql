-- migrations/007_create_trip_stops.sql
-- +goose Up

CREATE TABLE trip_stops (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trip_id         UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    location        GEOMETRY(Point, 4326) NOT NULL,
    address         VARCHAR(255) NOT NULL,
    arrival_time    TIMESTAMPTZ NOT NULL,
    sequence_number INT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_trip_stops_trip_id ON trip_stops(trip_id);

ALTER TABLE trip_passengers ADD COLUMN stop_id UUID REFERENCES trip_stops(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE trip_passengers DROP COLUMN stop_id;
DROP TABLE trip_stops;
