package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"api/internal/httpapi/internal/entity"
	"api/internal/httpapi/internal/middleware"
	"api/internal/httpapi/internal/store"
)

// MediaMTXAuth godoc
// @Summary      Authorize a MediaMTX request
// @Description  Allows publish, read, and playback actions for registered stream keys
// @Tags         mediamtx
// @Accept       json
// @Param        body  body  entity.MediaMTXAuthRequest  true  "MediaMTX auth payload"
// @Success      204
// @Failure      400  {object}  entity.ErrorResponse
// @Failure      401  {object}  entity.ErrorResponse
// @Failure      403  {object}  entity.ErrorResponse
// @Failure      500  {object}  entity.ErrorResponse
// @Router       /mediamtx/auth [post]
func (h *Handler) MediaMTXAuth(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var body entity.MediaMTXAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return middleware.BadRequest("invalid MediaMTX auth request")
	}

	if err := h.stream.AuthorizeMediaMTX(r.Context(), body); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return middleware.Unauthorized("stream is not authorized")
		}
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// MediaMTXHook godoc
// @Summary      Receive a MediaMTX lifecycle hook
// @Description  Updates stream state for ready and not-ready events and acknowledges informational events
// @Tags         mediamtx
// @Accept       application/x-www-form-urlencoded
// @Param        event  path      string  true  "Hook event"  Enums(init, ready, not-ready, read, unread)
// @Param        path   formData  string  true  "Stream key"
// @Success      204
// @Failure      400  {object}  entity.ErrorResponse
// @Failure      404  {object}  entity.ErrorResponse
// @Failure      500  {object}  entity.ErrorResponse
// @Router       /mediamtx/hooks/{event} [post]
func (h *Handler) MediaMTXHook(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		return middleware.BadRequest("invalid MediaMTX hook payload")
	}

	req := entity.MediaMTXHookRequest{
		Event: r.PathValue("event"),
		Path:  r.FormValue("path"),
	}
	if err := h.stream.HandleMediaMTXHook(r.Context(), req); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
