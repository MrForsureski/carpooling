-- migrations/008_add_city_to_users.sql
-- +goose Up

ALTER TABLE users ADD COLUMN IF NOT EXISTS city VARCHAR(100);

-- +goose Down

ALTER TABLE users DROP COLUMN IF EXISTS city;
