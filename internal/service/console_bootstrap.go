package service

import (
	"context"

	"nairo/api_service/ent/user"
	"nairo/api_service/internal/config"
	"nairo/api_service/internal/logger"
	"nairo/api_service/internal/mysql"

	"go.uber.org/fx"
)

const (
	demoLogin    = "user01"
	demoPassword = "user01"
	demoEmail    = "user01@nairo.local"
)

// BootstrapConsoleDemo 若尚无演示商户则创建 user01 / user01，并预充 100 元手续费余额。
func BootstrapConsoleDemo(lc fx.Lifecycle, cfg *config.Config, db *mysql.Client, users *UserAdminService, log *logger.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if cfg != nil && !cfg.Console.BootstrapDemo {
				log.Info(ctx, "console demo bootstrap skipped")
				return nil
			}
			n, err := db.Ent().User.Query().Where(user.PhoneEQ(demoLogin)).Count(ctx)
			if err != nil {
				return err
			}
			if n > 0 {
				return nil
			}
			res, err := users.CreateUser(ctx, CreateUserInput{
				Email:      demoEmail,
				Phone:      demoLogin,
				Password:   demoPassword,
				FeeRateBps: 100,
			})
			if err != nil {
				return err
			}
			if _, err := users.Recharge(ctx, res.User.ID, 0, 10000, "console demo seed"); err != nil {
				return err
			}
			log.Info(ctx, "console demo user created",
				"login", demoLogin,
				"email", demoEmail,
				"user_id", res.User.ID,
				"access_key", res.AccessKey,
			)
			return nil
		},
	})
}
