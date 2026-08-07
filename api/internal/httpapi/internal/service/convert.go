package service

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func parseKey(key string) (pgtype.UUID, error) {
	id, err := uuid.Parse(key)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid stream key %q: %w", key, err)
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

func newStreamKey() (pgtype.UUID, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}
