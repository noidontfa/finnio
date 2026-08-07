package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	"api/internal/httpapi/internal/entity"
	"api/internal/httpapi/internal/service"
	"api/internal/httpapi/internal/store"
	"api/internal/httpapi/internal/util"
)

// AppHandler is an HTTP handler that returns an error for centralized handling.
type AppHandler func(w http.ResponseWriter, r *http.Request) error

// APIError is an error with an HTTP status and client-facing message.
type APIError struct {
	Status  int
	Message string
	Err     error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *APIError) Unwrap() error {
	return e.Err
}

func BadRequest(msg string) error {
	return &APIError{Status: http.StatusBadRequest, Message: msg}
}

func Unauthorized(msg string) error {
	return &APIError{Status: http.StatusUnauthorized, Message: msg}
}

func Forbidden(msg string) error {
	return &APIError{Status: http.StatusForbidden, Message: msg}
}

func NotFound(msg string) error {
	return &APIError{Status: http.StatusNotFound, Message: msg}
}

func Conflict(msg string) error {
	return &APIError{Status: http.StatusConflict, Message: msg}
}

func ServiceUnavailable(msg string) error {
	return &APIError{Status: http.StatusServiceUnavailable, Message: msg}
}

func Internal(msg string) error {
	return &APIError{Status: http.StatusInternalServerError, Message: msg}
}

// Handler adapts an AppHandler to http.HandlerFunc for chi routes.
// Errors are mapped to JSON responses; 5xx are logged via the request logger.
func Handler(h AppHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			writeError(w, Logger(r), err)
		}
	}
}

func writeError(w http.ResponseWriter, log *slog.Logger, err error) {
	status, msg := mapError(err)
	if log != nil && status >= http.StatusInternalServerError {
		log.Error("request failed", "err", err, "status", status)
	}
	_ = util.WriteJson(w, status, entity.ErrorResponse{Msg: msg})
}

func mapError(err error) (status int, msg string) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status, apiErr.Message
	}

	switch {
	case errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, store.ErrConflict):
		return http.StatusConflict, err.Error()
	case errors.Is(err, service.ErrUnsupportedMediaMTXAction):
		return http.StatusForbidden, err.Error()
	case errors.Is(err, service.ErrStreamPathRequired):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, service.ErrUnknownMediaMTXHookEvent):
		return http.StatusNotFound, err.Error()
	default:
		return http.StatusInternalServerError, "Something went wrong. Please try again"
	}
}
