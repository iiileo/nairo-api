package v1

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

type wsHeartbeatMsg struct {
	Type       string `json:"type"`
	CallbackID string `json:"callback_id"`
	T          int64  `json:"t"`
	Nonce      string `json:"nonce"`
	Sign       string `json:"sign"`
	ConnType   string `json:"conn_type"`
	ConfigVer  string `json:"config_ver"`
	DeviceID   string `json:"device_id"`
}

// HeartbeatWS：App WebSocket 心跳。首包与后续包均使用与 HTTP 相同的签名字段。
func (h *AppPushHandler) HeartbeatWS(c *gin.Context) {
	up := h.wsUpgrader()
	conn, err := up.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	wsIdle := service.AppOnlineWindow + 10*time.Second
	_ = conn.SetReadDeadline(time.Now().Add(wsIdle))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(wsIdle))
		return nil
	})

	authed := false
	callbackID := ""
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(wsIdle))

		var msg wsHeartbeatMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			_ = conn.WriteJSON(gin.H{"ok": false, "error": "invalid_body"})
			continue
		}
		typ := strings.ToLower(strings.TrimSpace(msg.Type))
		if typ == "" || typ == "auth" || typ == "heartbeat" || typ == "ping" {
			// ok
		} else {
			_ = conn.WriteJSON(gin.H{"ok": false, "error": "unknown_type"})
			continue
		}
		if msg.ConnType == "" {
			msg.ConnType = "ws"
		}
		acc, err := h.acct.TouchHeartbeat(c.Request.Context(), service.AppHeartbeatInput{
			CallbackID: msg.CallbackID,
			T:          msg.T,
			Nonce:      msg.Nonce,
			Sign:       msg.Sign,
			ConnType:   msg.ConnType,
			DeviceID:   msg.DeviceID,
		})
		if err != nil {
			code := "invalid_request"
			switch {
			case errors.Is(err, service.ErrBoundElsewhere):
				code = "bound_elsewhere"
			case errors.Is(err, service.ErrUnauthorized):
				code = "bad_sign"
			case errors.Is(err, service.ErrNotFound):
				code = "not_found"
			}
			_ = conn.WriteJSON(gin.H{"ok": false, "error": code})
			if !authed {
				return
			}
			continue
		}
		authed = true
		callbackID = msg.CallbackID
		_ = conn.WriteJSON(h.heartbeatBody(c, acc, msg.ConfigVer, gin.H{
			"type":        "pong",
			"callback_id": callbackID,
			"conn_type":   "ws",
		}))
	}
}
