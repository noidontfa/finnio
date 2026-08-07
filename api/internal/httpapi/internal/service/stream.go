package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"api/gen/db"
	"api/internal/httpapi/internal/store"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Service) List(ctx context.Context) ([]db.Stream, error) {
	return s.q.ListStreams(ctx)
}

func (s *Service) Create(ctx context.Context, name string, status db.StreamStatus) (db.Stream, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return db.Stream{}, fmt.Errorf("name is required")
	}
	if status == "" {
		status = db.StreamStatusIdle
	}

	key, err := newStreamKey()
	if err != nil {
		return db.Stream{}, err
	}

	created, err := s.q.CreateStream(ctx, db.CreateStreamParams{
		Key:    key,
		Name:   name,
		Status: status,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.Stream{}, fmt.Errorf("stream already exists: %w", store.ErrConflict)
		}
		return db.Stream{}, err
	}
	return created, nil
}

func (s *Service) GetByID(ctx context.Context, id int32) (db.Stream, error) {
	row, err := s.q.GetStreamByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Stream{}, fmt.Errorf("stream %d not found: %w", id, store.ErrNotFound)
		}
		return db.Stream{}, err
	}
	return row, nil
}

func (s *Service) GetByKey(ctx context.Context, key string) (db.Stream, error) {
	pgKey, err := parseKey(key)
	if err != nil {
		return db.Stream{}, fmt.Errorf("stream %q not found: %w", key, store.ErrNotFound)
	}

	row, err := s.q.GetStreamByKey(ctx, pgKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Stream{}, fmt.Errorf("stream %q not found: %w", key, store.ErrNotFound)
		}
		return db.Stream{}, err
	}
	return row, nil
}

func (s *Service) Update(ctx context.Context, id int32, name string, status db.StreamStatus) (db.Stream, error) {
	existing, err := s.GetByID(ctx, id)
	if err != nil {
		return db.Stream{}, err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = existing.Name
	}
	if status == "" {
		status = existing.Status
	}

	updated, err := s.q.UpdateStream(ctx, db.UpdateStreamParams{
		ID:     id,
		Name:   name,
		Status: status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Stream{}, fmt.Errorf("stream %d not found: %w", id, store.ErrNotFound)
		}
		return db.Stream{}, err
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id int32) error {
	n, err := s.q.DeleteStream(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("stream %d not found: %w", id, store.ErrNotFound)
	}
	return nil
}

func (s *Service) Ready(ctx context.Context, key string) (db.Stream, error) {
	return s.updateStatusByKey(ctx, key, db.StreamStatusReady)
}

func (s *Service) GoLive(ctx context.Context, key string) (db.Stream, error) {
	return s.updateStatusByKey(ctx, key, db.StreamStatusLive)
}

func (s *Service) Pause(ctx context.Context, key string) (db.Stream, error) {
	return s.updateStatusByKey(ctx, key, db.StreamStatusPaused)
}

func (s *Service) Resume(ctx context.Context, key string) (db.Stream, error) {
	return s.updateStatusByKey(ctx, key, db.StreamStatusLive)
}

func (s *Service) End(ctx context.Context, key string) (db.Stream, error) {
	return s.updateStatusByKey(ctx, key, db.StreamStatusEnded)
}

func (s *Service) updateStatusByKey(ctx context.Context, key string, status db.StreamStatus) (db.Stream, error) {
	pgKey, err := parseKey(key)
	if err != nil {
		return db.Stream{}, fmt.Errorf("stream %q not found: %w", key, store.ErrNotFound)
	}

	updated, err := s.q.UpdateStreamStatusByKey(ctx, db.UpdateStreamStatusByKeyParams{
		Key:    pgKey,
		Status: status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Stream{}, fmt.Errorf("stream %q not found: %w", key, store.ErrNotFound)
		}
		return db.Stream{}, err
	}
	return updated, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
