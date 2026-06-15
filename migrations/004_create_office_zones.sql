-- migrations/004_create_office_zones.sql
-- +goose Up

CREATE TABLE office_zones (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    office_id           UUID NOT NULL REFERENCES offices(id) ON DELETE CASCADE,
    pickup_zone         GEOMETRY(Polygon, 4326) NOT NULL,
    max_detour_minutes  INTEGER NOT NULL DEFAULT 15
                            CHECK (max_detour_minutes BETWEEN 1 AND 60),
    max_distance_meters INTEGER NOT NULL DEFAULT 2000
                            CHECK (max_distance_meters BETWEEN 100 AND 10000),
    min_join_minutes    INTEGER NOT NULL DEFAULT 30
                            CHECK (min_join_minutes BETWEEN 5 AND 180),
    min_cancel_minutes  INTEGER NOT NULL DEFAULT 30
                            CHECK (min_cancel_minutes BETWEEN 5 AND 180),
    max_seats           INTEGER NOT NULL DEFAULT 6
                            CHECK (max_seats BETWEEN 1 AND 20),
    is_active           BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_office_zones_office_id ON office_zones(office_id);
CREATE INDEX idx_office_zones_pickup_zone ON office_zones USING GIST(pickup_zone);

-- +goose Down
DROP TABLE office_zones;
