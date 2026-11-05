-- +goose Up
ALTER TABLE users
    ADD CONSTRAINT users_role_valid CHECK (role IN ('VIEWER', 'CREATOR', 'ADMIN'));
ALTER TABLE videos
    ADD CONSTRAINT videos_status_valid CHECK (status IN ('PENDING', 'PROCESSING', 'PUBLISHED', 'FAILED'));

-- +goose Down
ALTER TABLE videos DROP CONSTRAINT IF EXISTS videos_status_valid;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_valid;
