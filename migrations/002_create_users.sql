-- migrations/002_create_users.sql
-- +goose Up

CREATE TYPE user_role AS ENUM ('unverified', 'passenger', 'driver', 'admin');

CREATE TABLE users (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    company_id       UUID NOT NULL REFERENCES companies(id) ON DELETE RESTRICT,
    email            VARCHAR(255) NOT NULL UNIQUE,
    password_hash    VARCHAR(255) NOT NULL,
    full_name        VARCHAR(255) NOT NULL,
    phone            VARCHAR(20),
    role             user_role NOT NULL DEFAULT 'unverified',
    email_verified   BOOLEAN NOT NULL DEFAULT false,
    verify_token     VARCHAR(255),
    verify_token_at  TIMESTAMPTZ,
    refresh_token    VARCHAR(255),
    refresh_token_at TIMESTAMPTZ,
    is_active        BOOLEAN NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_company_id ON users(company_id);
CREATE INDEX idx_users_role ON users(role);

-- +goose Down
DROP TABLE users;
DROP TYPE user_role;
