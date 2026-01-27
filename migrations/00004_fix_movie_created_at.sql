-- +goose Up
-- +goose StatementBegin
ALTER TABLE movies
ALTER COLUMN created_at TYPE TIMESTAMP WITH TIME ZONE;

ALTER TABLE movies
ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE movies
ALTER COLUMN created_at TYPE TIMESTAMP(0) WITH TIME ZONE NOT NULL;

ALTER TABLE movies
ALTER COLUMN created_at SET DEFAULT NOW();
-- +goose StatementEnd
