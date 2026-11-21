package v1

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"nairo/api_service/ent"
	"nairo/api_service/internal/config"
	"nairo/api_service/internal/logger"
	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type AppPushHandler struct {
	push *service.AppPushService
	acct *service.AccountService
	bot  *service.BotNotifyService
	cfg  *config.Config
	log  *logger.Logger
}

func NewAppPushHandler(push *service.AppPushService, acct *service.AccountService, bot *service.BotNotifyService, cfg *config.Config, log *logger.Logger) *AppPushHandler {
	return &AppPushHandler{push: push, acct: acct, bot: bot, cfg: cfg, log: log}
}

// heartbeatBody 心跳成功响应：始终带 config_ver；与客户端 config_ver 不一致时附带完整 config。
func (h *AppPushHandler) heartbeatBody(c *gin.Context, acc *ent.PaymentAccount, clientVer string, base gin.H) gin.H {
	if base == nil {
		base = gin.H{}
	}
	base["ok"] = true
	if h.bot == nil || acc == nil {
		return base
	}
	rc, err := h.bot.BuildAppRuntimeConfig(c.Request.Context(), acc)
	if err != nil {
		return base
	}
	base["config_ver"] = rc.Version
	if strings.TrimSpace(clientVer) != rc.Version {
		base["config"] = rc
		h.log.Info(c.Request.Context(), "app config pushed on heartbeat",
			"account_id", acc.ID, "callback_id", acc.CallbackID,
			"client_ver", clientVer, "ver", rc.Version,
			"channels", rc.Account.Channels, "notify_channels", len(rc.NotifyChannels))
	}
	return base
}

func (h *AppPushHandler) wsUpgrader() websocket.Upgrader {
	allowed := map[string]struct{}{}
	if h.cfg != nil {
		for _, o := range strings.Split(h.cfg.Server.AllowedOrigins, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				allowed[o] = struct{}{}
			}
		}
	}
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// 未配置白名单时：prod 拒绝跨域 Origin，local 放宽
			if len(allowed) == 0 {
				if h.cfg != nil && (h.cfg.Server.Mode == "prod" || h.cfg.Server.Mode == "production") {
					origin := r.Header.Get("Origin")
					return origin == "" // 原生 App 通常无 Origin
				}
				return true
			}
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" {
				return true
			}
			_, ok := allowed[origin]
			return ok
		},
	}
}

type appPushReq struct {
	T        int64  `json:"t" binding:"required"`
	Channel  string `json:"channel" binding:"required"`
	Price    string `json:"price" binding:"required"`
	Nonce    string `json:"nonce" binding:"required"`
	Sign     string `json:"sign" binding:"required"`
	IdemKey  string `json:"idem_key"`
	DeviceID string `json:"device_id"`
}

func (h *AppPushHandler) Push(c *gin.Context) {
	callbackID := c.Param("callback_id")
	raw, _ := io.ReadAll(c.Request.Body)
	var req appPushReq
	if err := json.Unmarshal(raw, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": "invalid_body"})
		return
	}
	err := h.push.Handle(c.Request.Context(), service.AppPushInput{
		CallbackID: callbackID,
		T:          req.T, Channel: req.Channel, Price: req.Price, Nonce: req.Nonce, Sign: req.Sign,
		IdemKey: req.IdemKey, DeviceID: req.DeviceID,
		RawJSON: string(raw),
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUnauthorized):
			c.JSON(http.StatusUnauthorized, gin.H{"code": 1, "error": "bad_sign"})
		case errors.Is(err, service.ErrBoundElsewhere):
			c.JSON(http.StatusConflict, gin.H{"code": 1, "error": "bound_elsewhere"})
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": 1, "error": "unknown_callback"})
		case errors.Is(err, service.ErrInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": err.Error()})
		default:
			h.log.Error(c.Request.Context(), "app push failed",
				"callback_id", callbackID, "channel", req.Channel, "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": "server_error"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

type heartbeatReq struct {
	CallbackID string `json:"callback_id" binding:"required"`
	T          int64  `json:"t" binding:"required"`
	Nonce      string `json:"nonce" binding:"required"`
	Sign       string `json:"sign" binding:"required"`
	ConnType   string `json:"conn_type"`
	ConfigVer  string `json:"config_ver"`
	DeviceID   string `json:"device_id"`
}

func (h *AppPushHandler) Heartbeat(c *gin.Context) {
	var req heartbeatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	acc, err := h.acct.TouchHeartbeat(c.Request.Context(), service.AppHeartbeatInput{
		CallbackID: req.CallbackID,
		T:          req.T,
		Nonce:      req.Nonce,
		Sign:       req.Sign,
		ConnType:   req.ConnType,
		DeviceID:   req.DeviceID,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBoundElsewhere):
			c.JSON(http.StatusConflict, gin.H{"error": "bound_elsewhere"})
		case errors.Is(err, service.ErrUnauthorized):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "bad_sign"})
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		case errors.Is(err, service.ErrInvalid):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		}
		return
	}
	c.JSON(http.StatusOK, h.heartbeatBody(c, acc, req.ConfigVer, nil))
}

type appBindReq struct {
	CallbackID string `json:"callback_id" binding:"required"`
	T          int64  `json:"t" binding:"required"`
	Nonce      string `json:"nonce" binding:"required"`
	Sign       string `json:"sign" binding:"required"`
	DeviceID   string `json:"device_id" binding:"required"`
	DeviceName string `json:"device_name"`
}

func (h *AppPushHandler) appBindInput(c *gin.Context) (service.AppBindInput, bool) {
	var req appBindReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid_body"})
		return service.AppBindInput{}, false
	}
	return service.AppBindInput{
		CallbackID: req.CallbackID, T: req.T, Nonce: req.Nonce, Sign: req.Sign,
		DeviceID: req.DeviceID, DeviceName: req.DeviceName,
	}, true
}

func (h *AppPushHandler) writeBindErr(c *gin.Context, err error, acc *ent.PaymentAccount) {
	switch {
	case errors.Is(err, service.ErrBoundElsewhere):
		name := ""
		if acc != nil {
			name = acc.BoundDeviceName
		}
		c.JSON(http.StatusConflict, gin.H{"ok": false, "error": "bound_elsewhere", "bound_device_name": name})
	case errors.Is(err, service.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error": "bad_sign"})
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error": "not_found"})
	case errors.Is(err, service.ErrInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid_request"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": "server_error"})
	}
}

// Bind 手机绑定监控设备（一个设备只能被一部手机绑定）。
func (h *AppPushHandler) Bind(c *gin.Context) {
	in, ok := h.appBindInput(c)
	if !ok {
		return
	}
	acc, err := h.acct.BindDevice(c.Request.Context(), in)
	if err != nil {
		h.writeBindErr(c, err, acc)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Unbind 手机解除绑定，释放监控设备。
func (h *AppPushHandler) Unbind(c *gin.Context) {
	in, ok := h.appBindInput(c)
	if !ok {
		return
	}
	if err := h.acct.UnbindDevice(c.Request.Context(), in); err != nil {
		h.writeBindErr(c, err, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
