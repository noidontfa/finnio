package main

import (
	"api/gen/db"
	_ "api/gen/swagger"
	"api/internal/config"
	"api/internal/httpapi"
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"shared/platform"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

// @title           Finnio API
// @version         1.0
// @description     Stream management API for Finnio.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@finnio.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:5555
// @BasePath  /

func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func NewDatabase(lc fx.Lifecycle, platform platform.Platform) (*db.Queries, error) {
	pool, err := pgxpool.New(context.Background(), platform.DatabaseURL)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			pool.Close()
			return nil
		},
	})

	return db.New(pool), nil
}

func main() {

	appModule := fx.Module(
		"all_modules",
		fx.Provide(
			platform.Load,
			NewDatabase,
			func(platform platform.Platform) config.Config {
				return config.New(platform.IngressURL)
			},
			NewLogger,
			httpapi.NewStreamService,
			httpapi.NewHandler,
			httpapi.NewServer,
		),
	)

	app := fx.New(
		appModule,
		fx.Invoke(func(srv *httpapi.Server, cfg config.Config, logr *slog.Logger) {

			httpSrv := &http.Server{
				Addr:              cfg.Addr,
				Handler:           srv.Routes(),
				ReadHeaderTimeout: 5 * time.Second,
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			go func() {
				logr.Info("api server starting", "addr", cfg.Addr)

				logr.Info("swagger url", "url", "/swagger/index.html")
				if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatal(err)
				}
			}()

			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = httpSrv.Shutdown(shutdownCtx)
			logr.Info("api stopped")

		}),
	)

	app.Run()

}
