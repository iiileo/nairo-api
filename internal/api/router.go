package api

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"nairo/api_service/internal/admin"
	"nairo/api_service/internal/api/middleware"
	v1 "nairo/api_service/internal/api/v1"
	"nairo/api_service/internal/config"
	"nairo/api_service/internal/console"
	"nairo/api_service/internal/service"
	"nairo/api_service/internal/webembed"

	"github.com/gin-gonic/gin"
)

type Router struct {
	engine *gin.Engine
}

func NewRouter(
	cfg *config.Config,
	health *v1.HealthHandler,
	bundles *webembed.Bundles,
	adminTM *admin.TokenManager,
	consoleTM *console.TokenManager,
	adminAuth *v1.AdminAuthHandler,
	adminUsers *v1.AdminUserHandler,
	adminPlatform *v1.AdminPlatformHandler,
	appMatchRules *v1.AppMatchRulesHandler,
	openOrders *v1.OpenOrderHandler,
	publicCheckout *v1.PublicCheckoutHandler,
	publicDemo *v1.PublicDemoHandler,
	publicGeetest *v1.PublicGeetestHandler,
	appPush *v1.AppPushHandler,
	manual *v1.ManualHandler,
	consoleAuth *v1.ConsoleAuthHandler,
	consoleMerchant *v1.ConsoleMerchantHandler,
	consoleRecharge *v1.ConsoleRechargeHandler,
	notifyJobs *v1.NotifyJobHandler,
	exportH *v1.ExportHandler,
	ops *v1.OpsHandler,
	orderSvc *service.OrderService,
) *Router {
	if cfg.Server.Mode == "local" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery(), gin.Logger())

	engine.GET("/healthz", health.Health)
	engine.GET("/version", health.Version)

	engine.POST("/app/push/:callback_id", middleware.RateLimit(5, 20), appPush.Push)
	engine.POST("/app/heartbeat", middleware.RateLimit(10, 40), appPush.Heartbeat)
	engine.GET("/app/heartbeat/ws", appPush.HeartbeatWS)
	engine.POST("/app/bind", middleware.RateLimit(2, 10), appPush.Bind)
	engine.POST("/app/unbind", middleware.RateLimit(2, 10), appPush.Unbind)
	engine.GET("/checkout/:order_id", publicCheckout.Page)
	engine.GET("/demo/:order_id", publicDemo.Page)

	apiV1 := engine.Group("/api/v1")
	{
		apiV1.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"pong": true})
		})
		apiV1.GET("/public/checkout/:order_id", publicCheckout.GetJSON)
		apiV1.GET("/public/qr.png", v1.PublicQR)
		apiV1.GET("/public/app-match-rules", appMatchRules.PublicGet)
		apiV1.GET("/public/demo-pay/status", publicDemo.Status)
		apiV1.POST("/public/demo-pay", publicDemo.Create)
		apiV1.GET("/public/demo-pay/:order_id", publicDemo.Get)
		apiV1.GET("/public/geetest", publicGeetest.Get)

		open := apiV1.Group("/open")
		open.Use(middleware.RateLimit(8, 30), middleware.OpenSign(orderSvc))
		{
			open.POST("/orders", openOrders.Create)
			open.GET("/orders/:out_trade_no", openOrders.Get)
		}

		consoleG := apiV1.Group("/console")
		{
			consoleG.POST("/login", middleware.RateLimit(1, 5), consoleAuth.Login)
			consoleG.POST("/register/send-code", middleware.RateLimit(0.2, 3), consoleAuth.SendRegisterCode)
			consoleG.POST("/register", middleware.RateLimit(0.2, 3), consoleAuth.Register)
			consoleG.POST("/refresh", consoleAuth.Refresh)
			consoleG.POST("/logout", consoleAuth.Logout)
			cz := consoleG.Group("")
			cz.Use(middleware.ConsoleAuth(consoleTM))
			{
				cz.GET("/me", consoleAuth.Me)
				cz.GET("/dashboard", consoleMerchant.Dashboard)
				cz.POST("/heartbeat", consoleAuth.Heartbeat)
				cz.POST("/adjust", consoleMerchant.UpdateAdjust)

				cz.GET("/accounts", consoleMerchant.ListAccounts)
				cz.POST("/accounts", consoleMerchant.CreateAccount)
				cz.GET("/accounts/:id/bind-config", consoleMerchant.GetBindConfig)
				cz.POST("/accounts/:id/reset-app-secret", consoleMerchant.ResetAppSecret)
				cz.POST("/accounts/:id/channels", consoleMerchant.UpdateChannels)
				cz.DELETE("/accounts/:id", consoleMerchant.DeleteAccount)
				cz.POST("/accounts/:id/release-phone", consoleMerchant.ReleasePhone)
				cz.GET("/accounts/:id/pay-codes", consoleMerchant.ListPayCodes)
				cz.POST("/pay-codes", consoleMerchant.CreatePayCode)
				cz.DELETE("/pay-codes/:id", consoleMerchant.DeletePayCode)

				cz.GET("/wallet/ledgers", consoleMerchant.ListLedgers)
				cz.GET("/wallet/pay-status", consoleRecharge.PlatformStatus)
				cz.POST("/wallet/pay", consoleRecharge.Create)
				cz.GET("/wallet/pay/:id", consoleRecharge.Get)
				cz.POST("/orders/test", middleware.RateLimit(1, 5), consoleMerchant.CreateTestOrder)
				cz.GET("/credential", consoleMerchant.GetCredential)
				cz.POST("/credential/reset", consoleMerchant.ResetCredential)

				cz.GET("/notify-channels", consoleMerchant.ListNotifyChannels)
				cz.POST("/notify-channels", consoleMerchant.CreateNotifyChannel)
				cz.PUT("/notify-channels/:id", consoleMerchant.UpdateNotifyChannel)
				cz.DELETE("/notify-channels/:id", consoleMerchant.DeleteNotifyChannel)
				cz.POST("/notify-channels/:id/test", consoleMerchant.TestNotifyChannel)

				cz.GET("/orders", manual.ListOrdersConsole)
				cz.POST("/orders/batch-status", manual.BatchSetStatusConsole)
				cz.POST("/orders/:id/manual-pay", manual.ManualPayConsole)
				cz.POST("/orders/:id/status", manual.SetStatusConsole)
				cz.GET("/orphans", manual.ListOrphansConsole)
				cz.POST("/orphans/batch-ignore", manual.BatchIgnoreOrphansConsole)
				cz.POST("/orphans/:id/bind", manual.BindOrphanConsole)
				cz.POST("/orphans/:id/ignore", manual.IgnoreOrphanConsole)

				cz.GET("/notify-jobs", notifyJobs.ListConsole)
				cz.POST("/notify-jobs/:id/replay", notifyJobs.ReplayConsole)
				cz.GET("/exports/orders.csv", exportH.OrdersCSVConsole)
				cz.GET("/exports/orphans.csv", exportH.OrphansCSVConsole)
				cz.GET("/exports/wallet-ledgers.csv", exportH.LedgersCSVConsole)
			}
		}

		if cfg.Admin.Enabled {
			adm := apiV1.Group("/admin")
			{
				adm.POST("/login", middleware.RateLimit(1, 5), adminAuth.Login)
				authz := adm.Group("")
				authz.Use(middleware.AdminAuth(adminTM))
				{
					authz.GET("/users", adminUsers.List)
					authz.POST("/users", adminUsers.Create)
					authz.POST("/users/:id", adminUsers.Update)
					authz.POST("/users/:id/password", adminUsers.SetPassword)
					authz.POST("/users/:id/fee-rate", adminUsers.SetFee)
					authz.POST("/users/:id/recharge", adminUsers.Recharge)
					authz.GET("/platform-settings", adminPlatform.GetSettings)
					authz.POST("/platform-settings/pay-user", adminPlatform.SetPayUser)
					authz.GET("/platform-settings/match-rules", adminPlatform.GetMatchRules)
					authz.PUT("/platform-settings/match-rules", adminPlatform.SetMatchRules)
					authz.POST("/platform-settings/match-rules/reset", adminPlatform.ResetMatchRules)
					authz.GET("/platform-settings/runtime", adminPlatform.GetRuntimeConfig)
					authz.PUT("/platform-settings/runtime", adminPlatform.SetRuntimeConfig)
					authz.POST("/platform-settings/runtime/reload", adminPlatform.ReloadRuntimeConfig)
					authz.POST("/accounts", adminUsers.CreateAccount)
					authz.POST("/pay-codes", adminUsers.CreatePayCode)
					authz.GET("/orders", manual.ListOrdersAdmin)
					authz.POST("/orders/batch-status", manual.BatchSetStatusAdmin)
					authz.POST("/orders/:id/manual-pay", manual.ManualPayAdmin)
					authz.POST("/orders/:id/status", manual.SetStatusAdmin)
					authz.GET("/orphans", manual.ListOrphansAdmin)
					authz.POST("/orphans/batch-ignore", manual.BatchIgnoreOrphansAdmin)
					authz.POST("/orphans/:id/bind", manual.BindOrphanAdmin)
					authz.POST("/orphans/:id/ignore", manual.IgnoreOrphanAdmin)
					authz.GET("/notify-jobs", notifyJobs.ListAdmin)
					authz.POST("/notify-jobs/:id/replay", notifyJobs.ReplayAdmin)
					authz.GET("/exports/orders.csv", exportH.OrdersCSVAdmin)
					authz.GET("/exports/orphans.csv", exportH.OrphansCSVAdmin)
					authz.GET("/ops/summary", ops.Summary)
				}
			}
		}
	}

	mountSPA(engine, "/console", bundles.Console)
	mountSPA(engine, cfg.Web.AdminBasePath, bundles.Admin)
	mountIndex(engine, bundles.Index, cfg.Web.AdminBasePath)

	return &Router{engine: engine}
}

func (r *Router) Engine() *gin.Engine { return r.engine }

func mountSPA(engine *gin.Engine, base string, bundle fs.FS) {
	base = strings.TrimSuffix(base, "/")
	if base == "" {
		base = "/"
	}
	engine.GET(base, spaFallback(base, bundle))
	engine.GET(base+"/*filepath", spaFallback(base, bundle))
	engine.HEAD(base, spaFallback(base, bundle))
	engine.HEAD(base+"/*filepath", spaFallback(base, bundle))
}

func spaFallback(base string, bundle fs.FS) gin.HandlerFunc {
	return func(c *gin.Context) {
		rel := strings.TrimPrefix(c.Request.URL.Path, base)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || strings.HasSuffix(rel, "/") {
			rel = strings.TrimSuffix(rel, "/")
			if rel == "" {
				rel = "index.html"
			} else {
				rel = rel + "/index.html"
			}
		}
		if st, err := fs.Stat(bundle, rel); err == nil && st.IsDir() {
			rel = strings.TrimSuffix(rel, "/") + "/index.html"
		}
		if _, err := fs.Stat(bundle, rel); err != nil {
			rel = "index.html"
		}
		// 不用 FileFromFS / ServeFile：它们会对 */index.html 301 到 ./，在二级目录下会循环
		serveEmbedFile(c, bundle, rel)
	}
}

func serveEmbedFile(c *gin.Context, bundle fs.FS, name string) {
	f, err := bundle.Open(name)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		data, readErr := io.ReadAll(f)
		if readErr != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Data(http.StatusOK, mimeByExt(name), data)
		return
	}
	http.ServeContent(c.Writer, c.Request, path.Base(name), st.ModTime(), rs)
}

func mimeByExt(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}

func mountIndex(engine *gin.Engine, bundle fs.FS, adminBase string) {
	fileServer := http.FileServer(http.FS(bundle))
	adminBase = strings.TrimSuffix(adminBase, "/")
	engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/app") ||
			strings.HasPrefix(path, "/console") || strings.HasPrefix(path, "/checkout") ||
			strings.HasPrefix(path, "/demo") ||
			strings.HasPrefix(path, adminBase) ||
			path == "/healthz" || path == "/version" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		rel := strings.TrimPrefix(path, "/")
		if rel == "" {
			rel = "index.html"
		}
		if _, err := fs.Stat(bundle, rel); err != nil {
			if !strings.HasSuffix(rel, ".html") {
				if _, err2 := fs.Stat(bundle, rel+".html"); err2 == nil {
					c.FileFromFS(rel+".html", http.FS(bundle))
					return
				}
			}
			c.Status(http.StatusNotFound)
			return
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
