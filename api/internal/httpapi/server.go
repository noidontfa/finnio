package httpapi

import (
	"log/slog"
	"net/http"

	"api/gen/db"
	"api/internal/config"
	"api/internal/httpapi/internal/entity"
	"api/internal/httpapi/internal/handler"
	mid "api/internal/httpapi/internal/middleware"
	"api/internal/httpapi/internal/service"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
)

func NewHandler(cfg config.Config, log *slog.Logger, streamService service.IStreamService) *handler.Handler {
	return handler.New(cfg, log, streamService)
}

func NewStreamService(q *db.Queries, log *slog.Logger) service.IStreamService {
	return service.New(q, log)
}

type Server struct {
	cfg   config.Config
	log   *slog.Logger
	hdler *handler.Handler
}

func NewServer(cfg config.Config, log *slog.Logger, han *handler.Handler) *Server {
	return &Server{
		cfg:   cfg,
		log:   log,
		hdler: han,
	}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(
		chimw.RequestID,
		chimw.RealIP,
		chimw.Recoverer,
		mid.RequestLogger(s.log),
	)

	r.Mount("/swagger", httpSwagger.WrapHandler)

	r.Get("/healthz", mid.Handler(s.hdler.Healthz))

	r.Route("/mediamtx", func(r chi.Router) {
		r.Post("/auth", mid.Handler(s.hdler.MediaMTXAuth))
		r.Post("/hooks/{event}", mid.Handler(s.hdler.MediaMTXHook))
	})

	r.Route("/streams", func(r chi.Router) {
		r.Get("/", mid.Handler(s.hdler.ListStreams))
		r.With(mid.ValidateBody[entity.CreateStreamRequest]()).
			Post("/", mid.Handler(s.hdler.CreateStream))

		r.Get("/{id}", mid.Handler(s.hdler.GetStream))
		r.With(mid.ValidateBody[entity.UpdateStreamRequest]()).
			Put("/{id}", mid.Handler(s.hdler.UpdateStream))
		r.Delete("/{id}", mid.Handler(s.hdler.DeleteStream))

		r.Post("/{key}/ready", mid.Handler(s.hdler.ReadyStream))
		r.Post("/{key}/go-live", mid.Handler(s.hdler.GoLiveStream))
		r.Post("/{key}/pause", mid.Handler(s.hdler.PauseStream))
		r.Post("/{key}/resume", mid.Handler(s.hdler.ResumeStream))
		r.Post("/{key}/end", mid.Handler(s.hdler.EndStream))
		r.Get("/{key}/ingress", mid.Handler(s.hdler.GetIngressURL))
	})

	r.Get("/hls/{key}/*", mid.Handler(s.hdler.ServeABRHLS))

	return r
}
