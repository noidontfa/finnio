package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
)

var v = validator.New()

type (
	bodyCtxKey  struct{}
	queryCtxKey struct{}
	pathCtxKey  struct{}
)

// Body returns the request body previously bound by ValidateBody.
func Body[T any](r *http.Request) (T, bool) {
	val, ok := r.Context().Value(bodyCtxKey{}).(T)
	return val, ok
}

// Query returns query params previously bound by ValidateQuery.
func Query[T any](r *http.Request) (T, bool) {
	val, ok := r.Context().Value(queryCtxKey{}).(T)
	return val, ok
}

// Path returns path params previously bound by ValidatePath.
func Path[T any](r *http.Request) (T, bool) {
	val, ok := r.Context().Value(pathCtxKey{}).(T)
	return val, ok
}

// ValidateBody decodes JSON into T, runs struct validation, and stores the
// value on the request context for Body[T]. Chi usage:
//
//	r.With(middleware.ValidateBody[entity.CreateStreamRequest]()).Post("/", h)
func ValidateBody[T any]() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body T
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&body); err != nil {
				writeError(w, Logger(r), BadRequest("invalid json body"))
				return
			}
			if err := v.StructCtx(r.Context(), body); err != nil {
				writeError(w, Logger(r), BadRequest(err.Error()))
				return
			}
			ctx := context.WithValue(r.Context(), bodyCtxKey{}, body)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ValidateQuery binds URL query values into T, validates, and stores them for Query[T].
func ValidateQuery[T any]() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var query T
			queryBytes, err := json.Marshal(r.URL.Query())
			if err != nil {
				writeError(w, Logger(r), BadRequest(err.Error()))
				return
			}
			if err = json.Unmarshal(queryBytes, &query); err != nil {
				writeError(w, Logger(r), BadRequest(err.Error()))
				return
			}
			if err := v.StructCtx(r.Context(), query); err != nil {
				writeError(w, Logger(r), BadRequest(err.Error()))
				return
			}
			ctx := context.WithValue(r.Context(), queryCtxKey{}, query)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ValidatePath binds named path params into T (via json field names), validates,
// and stores them for Path[T]. Chi usage:
//
//	r.With(middleware.ValidatePath[entity.IDPath]("id")).Get("/{id}", h)
func ValidatePath[T any](params ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pathParamsMap := make(map[string]string, len(params))
			for _, p := range params {
				pathParamsMap[p] = r.PathValue(p)
			}

			pathParamsBytes, err := json.Marshal(pathParamsMap)
			if err != nil {
				writeError(w, Logger(r), BadRequest(err.Error()))
				return
			}

			var pathParams T
			if err := json.Unmarshal(pathParamsBytes, &pathParams); err != nil {
				writeError(w, Logger(r), BadRequest(err.Error()))
				return
			}
			if err := v.StructCtx(r.Context(), pathParams); err != nil {
				writeError(w, Logger(r), BadRequest(err.Error()))
				return
			}

			ctx := context.WithValue(r.Context(), pathCtxKey{}, pathParams)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
