package service

import (
	"context"
	"log/slog"

	"api/gen/db"
	"api/internal/httpapi/internal/entity"
)

type IStreamService interface {
	List(ctx context.Context) ([]db.Stream, error)
	Create(ctx context.Context, name string, status db.StreamStatus) (db.Stream, error)
	GetByID(ctx context.Context, id int32) (db.Stream, error)
	GetByKey(ctx context.Context, key string) (db.Stream, error)
	Update(ctx context.Context, id int32, name string, status db.StreamStatus) (db.Stream, error)
	Delete(ctx context.Context, id int32) error
	Ready(ctx context.Context, key string) (db.Stream, error)
	GoLive(ctx context.Context, key string) (db.Stream, error)
	Pause(ctx context.Context, key string) (db.Stream, error)
	Resume(ctx context.Context, key string) (db.Stream, error)
	End(ctx context.Context, key string) (db.Stream, error)
	AuthorizeMediaMTX(ctx context.Context, req entity.MediaMTXAuthRequest) error
	HandleMediaMTXHook(ctx context.Context, req entity.MediaMTXHookRequest) error
}

var _ IStreamService = (*Service)(nil)

type Service struct {
	q   *db.Queries
	log *slog.Logger
}

func New(q *db.Queries, log *slog.Logger) *Service {
	return &Service{
		q:   q,
		log: log,
	}
}
