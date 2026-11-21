package main

import (
	"nairo/api_service/internal/admin"
	"nairo/api_service/internal/api"
	v1 "nairo/api_service/internal/api/v1"
	"nairo/api_service/internal/config"
	"nairo/api_service/internal/console"
	"nairo/api_service/internal/logger"
	"nairo/api_service/internal/mailer"
	"nairo/api_service/internal/mysql"
	"nairo/api_service/internal/pkg/secretbox"
	"nairo/api_service/internal/runtimeconfig"
	"nairo/api_service/internal/service"
	"nairo/api_service/internal/timezone"
	"nairo/api_service/internal/webembed"

	"go.uber.org/fx"
)

func main() {
	timezone.Init()
	fx.New(
		fx.Provide(
			config.NewConfig,
			logger.NewLogger,
			mysql.NewEntClient,
			webembed.NewBundles,
			admin.NewTokenManager,
			console.NewTokenManager,
			runtimeconfig.NewRuntimeConfig,
			mailer.NewMailer,
			newSecretVault,

			service.NewAdminAuthService,
			service.NewUserAdminService,
			service.NewAccountService,
			service.NewBotNotifyService,
			service.NewHeartbeatWatchService,
			service.NewPlatformService,
			service.NewRechargeService,
			service.NewOrderService,
			service.NewNotifyService,
			service.NewAppPushService,
			service.NewManualService,
			service.NewConsoleAuthService,
			service.NewConsoleMerchantService,

			v1.NewHealthHandler,
			v1.NewAdminAuthHandler,
			v1.NewAdminUserHandler,
			v1.NewAdminPlatformHandler,
			v1.NewAppMatchRulesHandler,
			v1.NewOpenOrderHandler,
			v1.NewPublicCheckoutHandler,
			v1.NewPublicDemoHandler,
			v1.NewPublicGeetestHandler,
			v1.NewAppPushHandler,
			v1.NewManualHandler,
			v1.NewConsoleAuthHandler,
			v1.NewConsoleMerchantHandler,
			v1.NewConsoleRechargeHandler,
			v1.NewNotifyJobHandler,
			v1.NewExportHandler,
			v1.NewOpsHandler,
			api.NewRouter,
		),
		fx.Invoke(admin.Bootstrap),
		fx.Invoke(runtimeconfig.Bootstrap),
		fx.Invoke(service.BootstrapConsoleDemo),
		fx.Invoke(service.RegisterHeartbeatWatch),
		fx.Invoke(service.RegisterNotifyWorker),
		fx.Invoke(service.RegisterAmountLockCleanup),
		fx.Invoke(api.StartServer),
	).Run()
}

func newSecretVault(cfg *config.Config) (*secretbox.Vault, error) {
	return secretbox.New(cfg.Security.MasterKey)
}
