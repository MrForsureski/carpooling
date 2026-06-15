-- migrations/003_create_offices.sql
-- +goose Up

CREATE TABLE offices (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    name       VARCHAR(255) NOT NULL,
    address    TEXT NOT NULL,
    location   GEOMETRY(Point, 4326) NOT NULL,
    is_active  BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_offices_company_id ON offices(company_id);
CREATE INDEX idx_offices_location ON offices USING GIST(location);

-- +goose Down
DROP TABLE offices;
