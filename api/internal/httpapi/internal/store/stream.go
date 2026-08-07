package store

import (
	"context"
	"errors"

	"api/internal/httpapi/internal/entity"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// StreamStore is the persistence interface for streams.
// Swap jsonstore for a SQL implementation without changing the service layer.
type StreamStore interface {
	List(ctx context.Context) ([]entity.Stream, error)
	GetByID(ctx context.Context, id int) (entity.Stream, error)
	GetByKey(ctx context.Context, key string) (entity.Stream, error)
	Create(ctx context.Context, stream entity.Stream) (entity.Stream, error)
	Update(ctx context.Context, id int, stream entity.Stream) (entity.Stream, error)
	UpdateStatusByKey(ctx context.Context, key string, status entity.StreamStatus) (entity.Stream, error)
	Delete(ctx context.Context, id int) error
}
