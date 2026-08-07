-- name: ListStreams :many
SELECT id, key, name, status FROM streams ORDER BY id;

-- name: GetStreamByID :one
SELECT id, key, name, status FROM streams WHERE id = $1;

-- name: GetStreamByKey :one
SELECT id, key, name, status FROM streams WHERE key = $1;

-- name: CreateStream :one
INSERT INTO
    streams (key, name, status)
VALUES ($1, $2, $3) RETURNING id,
    key,
    name,
    status;

-- name: UpdateStream :one
UPDATE streams
SET
    name = $2,
    status = $3
WHERE
    id = $1 RETURNING id,
    key,
    name,
    status;

-- name: UpdateStreamStatusByKey :one
UPDATE streams
SET
    status = $2
WHERE
    key = $1 RETURNING id,
    key,
    name,
    status;

-- name: DeleteStream :execrows
DELETE FROM streams WHERE id = $1;