-- +goose Up
-- +goose StatementBegin
ALTER TABLE movies
ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE movies
DROP COLUMN updated_at;
-- +goose StatementEnd
