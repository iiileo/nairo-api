package api

import (
	"context"
	"net/http"
	"time"

	"nairo/api_service/internal/config"
	"nairo/api_service/internal/logger"
	"nairo/api_service/internal/version"

	"go.uber.org/fx"
)

func StartServer(lc fx.Lifecycle, cfg *config.Config, router *Router, log *logger.Logger) {
	srv := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           router.Engine(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info(ctx, "starting http server",
				"addr", cfg.Server.Address,
				"version", version.Version,
				"commit", version.Commit,
				"web_dev", cfg.Web.Dev,
				"admin_base", cfg.Web.AdminBasePath,
			)
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Error(context.Background(), "http server failed", "err", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info(ctx, "stopping http server")
			shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return srv.Shutdown(shutdownCtx)
		},
	})
}
