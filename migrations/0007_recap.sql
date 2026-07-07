-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN last_recap_sent_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN last_recap_sent_at;
-- +goose StatementEnd
