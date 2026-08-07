package service

import (
	"context"
	"errors"

	"api/internal/httpapi/internal/entity"
)

var (
	ErrUnsupportedMediaMTXAction = errors.New("unsupported MediaMTX action")
	ErrStreamPathRequired        = errors.New("stream path is required")
	ErrUnknownMediaMTXHookEvent  = errors.New("unknown MediaMTX hook event")
)

func (s *Service) AuthorizeMediaMTX(ctx context.Context, req entity.MediaMTXAuthRequest) error {
	switch req.Action {
	case "publish", "read", "playback":
	default:
		return ErrUnsupportedMediaMTXAction
	}

	if req.Path == "" {
		return ErrStreamPathRequired
	}

	_, err := s.GetByKey(ctx, req.StreamKey())
	return err
}

func (s *Service) HandleMediaMTXHook(ctx context.Context, req entity.MediaMTXHookRequest) error {
	if req.Path == "" {
		return ErrStreamPathRequired
	}

	// ABR republish paths must not mutate the source stream lifecycle.
	if entity.IsABRPath(req.Path) {
		s.log.Info("MediaMTX hook ignored for ABR path", "event", req.Event, "path", req.Path)
		return nil
	}

	switch req.Event {
	case "ready":
		_, err := s.GoLive(ctx, req.Path)
		return err
	case "not-ready":
		_, err := s.End(ctx, req.Path)
		return err
	case "init", "read", "unread":
		s.log.Info("MediaMTX hook", "event", req.Event, "path", req.Path)
		return nil
	default:
		return ErrUnknownMediaMTXHookEvent
	}
}
