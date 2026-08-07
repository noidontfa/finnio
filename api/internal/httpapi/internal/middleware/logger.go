package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

type loggerCtxKey struct{}

// Logger returns the request-scoped slog logger, or slog.Default if unset.
func Logger(r *http.Request) *slog.Logger {
	if log, ok := r.Context().Value(loggerCtxKey{}).(*slog.Logger); ok && log != nil {
		return log
	}
	return slog.Default()
}

// RequestLogger injects the logger into the request context and logs each request.
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), loggerCtxKey{}, log)
			r = r.WithContext(ctx)

			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration", time.Since(start),
				"request_id", chimw.GetReqID(r.Context()),
			)
		})
	}
}
