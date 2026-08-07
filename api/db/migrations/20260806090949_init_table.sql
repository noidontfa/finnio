-- +goose Up
CREATE TYPE stream_status AS ENUM (
    'idle',
    'ready',
    'live',
    'paused',
    'ended',
    'failed'
);

CREATE TABLE streams (
    id SERIAL PRIMARY KEY,
    key UUID NOT NULL UNIQUE,
    name TEXT NOT NULL,
    status stream_status NOT NULL DEFAULT 'idle'
);

-- +goose Down
DROP TABLE streams;

DROP TYPE stream_status;