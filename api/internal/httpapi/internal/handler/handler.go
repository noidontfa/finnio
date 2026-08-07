package handler

import (
	"log/slog"
	"net/http"

	"api/internal/config"
	"api/internal/httpapi/internal/service"
	"api/internal/httpapi/internal/util"
)

type Handler struct {
	cfg    config.Config
	log    *slog.Logger
	stream service.IStreamService
}

func New(cfg config.Config, log *slog.Logger, streamService service.IStreamService) *Handler {
	return &Handler{
		cfg:    cfg,
		log:    log,
		stream: streamService,
	}
}

func (h *Handler) response(w http.ResponseWriter, d any, status int) error {
	if err := util.WriteJson(w, status, d); err != nil {
		h.log.Error("write response", "err", err)
	}
	return nil
}
