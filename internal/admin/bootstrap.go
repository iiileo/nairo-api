package admin

import (
	"context"
	"strings"

	"nairo/api_service/internal/config"
	"nairo/api_service/internal/logger"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/pkg/secrethash"
	"nairo/api_service/internal/snowflake"

	"go.uber.org/fx"
)

func Bootstrap(lc fx.Lifecycle, cfg *config.Config, db *mysql.Client, log *logger.Logger) {
	if cfg == nil || !cfg.Admin.Enabled {
		return
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			n, err := db.Ent().Admin.Query().Count(ctx)
			if err != nil {
				return err
			}
			if n > 0 {
				return nil
			}
			hash, err := secrethash.HashPassword(cfg.Admin.BootstrapPassword)
			if err != nil {
				return err
			}
			id, err := snowflake.NextID()
			if err != nil {
				return err
			}
			a, err := db.Ent().Admin.Create().
				SetID(id).
				SetUsername(strings.TrimSpace(cfg.Admin.BootstrapUsername)).
				SetPasswordHash(hash).
				SetName("超级管理员").
				SetStatus("active").
				Save(ctx)
			if err != nil {
				return err
			}
			log.Info(ctx, "admin bootstrap account created", "username", a.Username, "id", a.ID)
			return nil
		},
	})
}
