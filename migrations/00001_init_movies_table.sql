-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS movies (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP(0) WITH time zone NOT NULL DEFAULT NOW(),
    title TEXT NOT NULL,
    year INTEGER NOT NULL,
    runtime integer NOT NULL,
    genres TEXT[] NOT NULL,
    version INTEGER NOT NULL DEFAULT 1
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS movies;
-- +goose StatementEnd
