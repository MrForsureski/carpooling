-- migrations/006_create_trip_passengers.sql
-- +goose Up

CREATE TYPE passenger_status AS ENUM (
    'confirmed',
    'cancelled'
);

CREATE TABLE trip_passengers (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trip_id        UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    pickup         GEOMETRY(Point, 4326) NOT NULL,
    pickup_address TEXT,
    detour_seconds INTEGER NOT NULL DEFAULT 0,
    status         passenger_status NOT NULL DEFAULT 'confirmed',
    joined_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cancelled_at   TIMESTAMPTZ,

    UNIQUE(trip_id, user_id)
);

CREATE INDEX idx_trip_passengers_trip_id ON trip_passengers(trip_id);
CREATE INDEX idx_trip_passengers_user_id ON trip_passengers(user_id);
CREATE INDEX idx_trip_passengers_pickup ON trip_passengers USING GIST(pickup);

-- +goose Down
DROP TABLE trip_passengers;
DROP TYPE passenger_status;
