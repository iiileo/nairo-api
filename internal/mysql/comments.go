package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// tableComments / columnComments：已有库 Schema.Create 不会改 COMMENT，启动时主动同步。
var tableComments = map[string]string{
	"admins":             "平台管理员",
	"users":              "商户用户",
	"api_credentials":    "商户 Open API 凭证",
	"payment_accounts":   "支付账户（微信/支付宝收款号）",
	"pay_codes":          "收款码",
	"orders":             "支付订单",
	"amount_locks":       "金额槽位锁（微调占额）",
	"wallets":            "商户手续费钱包",
	"wallet_ledgers":     "钱包流水",
	"payment_orphans":    "未匹配到账（孤儿收款）",
	"manual_settle_logs": "人工补单审计日志",
	"email_verifications": "邮箱验证码",
	"refresh_tokens":      "Console 刷新令牌",
	"notify_channels":     "商户通知机器人（企业微信/飞书/钉钉）",
	"platform_settings":   "平台配置",
	"idempotency_records":  "幂等键（推送/通知去重）",
	"merchant_notify_jobs": "商户支付结果回调任务（持久化重试）",
}

var columnComments = map[string]map[string]string{
	"admins": {
		"id": "主键（雪花 ID）", "username": "登录用户名", "password_hash": "密码哈希",
		"name": "显示名称", "status": "状态：active/disabled",
		"created_at": "创建时间", "updated_at": "更新时间",
	},
	"users": {
		"id": "主键（雪花 ID）", "email": "邮箱（可登录）", "phone": "手机号（可登录）",
		"password_hash": "密码哈希", "status": "状态：active/disabled/pending",
		"fee_rate_bps": "手续费率（基点，100=1%）", "adjust_enabled": "是否启用金额微调",
		"adjust_direction": "微调方向：inc 递增 / dec 递减", "adjust_step_cents": "微调步长（分）",
		"adjust_max_offset_cents": "微调最大偏移（分）", "console_last_seen_at": "控制台最近心跳时间",
		"created_at": "创建时间", "updated_at": "更新时间",
	},
	"api_credentials": {
		"id": "主键（雪花 ID）", "user_id": "所属商户用户 ID",
		"access_key": "Access Key（公开标识）", "secret_hash": "API Secret（HMAC 签名用，MVP 存明文）",
		"created_at": "创建时间", "updated_at": "更新时间",
	},
	"payment_accounts": {
		"id": "主键（雪花 ID）", "user_id": "所属商户用户 ID",
		"channel": "支持渠道（逗号分隔）：wechat/alipay，可同时支持",
		"name": "账户备注名", "callback_id": "App 回调短码", "app_secret_hash": "App 推送签名密钥",
		"status": "状态：active/disabled", "app_last_seen_at": "App 最近在线时间",
		"app_conn_type": "连接类型：http/ws", "created_at": "创建时间", "updated_at": "更新时间",
		"bound_device_id":   "已绑定手机的安装 ID（空=未绑定）",
		"bound_device_name": "已绑定手机型号（展示用）",
		"bound_at":          "手机绑定时间",
	},
	"pay_codes": {
		"id": "主键（雪花 ID）", "account_id": "所属支付账户 ID",
		"code_type": "类型：float 任意金额 / fixed 固额", "amount_cents": "固额金额（分），float 时可空",
		"image_url": "收款码图片 URL", "payload": "可选原始码内容", "status": "状态：active/disabled",
		"created_at": "创建时间", "updated_at": "更新时间",
	},
	"orders": {
		"id": "主键（雪花 ID）", "user_id": "收款商户用户 ID", "out_trade_no": "商户侧订单号",
		"purpose": "用途：collect 收款 / recharge 手续费充值 / test 在线测试",
		"payer_user_id": "付款/充值受益商户 ID（recharge/test）",
		"amount_cents": "标价金额（分）", "pay_amount_cents": "实付金额（分，含微调）",
		"adjust_offset_cents": "微调偏移（分）", "channel": "支付渠道：wechat/alipay",
		"status": "状态：pending/paid/expired/closed", "notify_url": "商户异步通知 URL", "return_url": "支付成功后收银台跳转 URL",
		"pay_code_id": "分配的收款码 ID", "account_id": "收款支付账户 ID",
		"settle_type": "入账方式：auto/manual", "manual_remark": "人工补单备注",
		"manual_by_type": "补单操作者类型：console/admin", "manual_by_id": "补单操作者 ID",
		"manual_at": "人工补单时间", "paid_at": "支付完成时间", "expire_at": "订单过期时间",
		"created_at": "创建时间", "updated_at": "更新时间",
	},
	"platform_settings": {
		"id": "主键（雪花 ID）", "key": "配置键", "value": "配置值（可存 JSON）",
		"updated_at": "更新时间", "created_at": "创建时间",
	},
	"amount_locks": {
		"id": "主键（雪花 ID）", "user_id": "商户用户 ID", "channel": "渠道：wechat/alipay",
		"pay_amount_cents": "锁定的实付金额（分）", "order_id": "关联订单 ID",
		"locked_until": "锁到期时间", "created_at": "创建时间",
	},
	"wallets": {
		"id": "主键（雪花 ID）", "user_id": "商户用户 ID", "balance_cents": "余额（分）",
		"created_at": "创建时间", "updated_at": "更新时间",
	},
	"notify_channels": {
		"id": "主键（雪花 ID）", "user_id": "所属商户用户 ID",
		"account_id": "绑定监控设备 ID；空=商户级（全部设备）",
		"provider": "渠道：wecom/feishu/dingtalk", "name": "备注名",
		"webhook_url": "Webhook URL", "secret": "加签密钥（钉钉/飞书可选）",
		"enabled": "是否启用", "event_order_paid": "订单入账通知",
		"event_fee_low": "手续费不足通知", "event_app_push": "收到 App 支付推送通知",
		"event_device_offline": "监控设备心跳丢失告警",
		"created_at": "创建时间", "updated_at": "更新时间",
	},
	"wallet_ledgers": {
		"id": "主键（雪花 ID）", "user_id": "商户用户 ID",
		"type": "类型：recharge/fee_deduct/adjust", "amount_cents": "变动金额（分，可正可负）",
		"balance_after_cents": "变动后余额（分）", "order_id": "关联订单 ID",
		"admin_id": "操作管理员 ID（充值时）", "remark": "备注", "created_at": "创建时间",
	},
	"payment_orphans": {
		"id": "主键（雪花 ID）", "user_id": "商户用户 ID", "account_id": "上报的支付账户 ID",
		"channel": "渠道：wechat/alipay", "pay_amount_cents": "上报金额（分）",
		"reported_at": "App 上报时间", "raw_payload": "原始上报载荷",
		"status": "状态：open/bound/ignored", "bound_order_id": "绑定的订单 ID",
		"created_at": "创建时间", "updated_at": "更新时间",
	},
	"manual_settle_logs": {
		"id": "主键（雪花 ID）", "order_id": "订单 ID", "user_id": "商户用户 ID",
		"actor_type": "操作者类型：console/admin", "actor_id": "操作者 ID",
		"action": "动作：bind_orphan/manual_pay", "orphan_id": "关联孤儿单 ID",
		"remark": "备注", "before_status": "操作前订单状态", "after_status": "操作后订单状态",
		"created_at": "创建时间",
	},
	"idempotency_records": {
		"id": "主键（雪花 ID）", "scope": "作用域：app_push/bot_notify",
		"idem_key": "幂等键", "result": "处理结果摘要", "created_at": "创建时间",
	},
	"merchant_notify_jobs": {
		"id": "主键（雪花 ID）", "user_id": "商户用户 ID", "order_id": "订单 ID",
		"out_trade_no": "商户订单号", "notify_url": "回调 URL", "payload": "POST JSON 正文",
		"status": "pending/done/failed", "attempts": "已尝试次数", "max_attempts": "最大尝试次数",
		"next_retry_at": "下次重试时间", "last_error": "最近错误",
		"created_at": "创建时间", "updated_at": "更新时间",
	},
	"email_verifications": {
		"id": "主键（雪花 ID）", "email": "邮箱", "code_hash": "验证码哈希",
		"purpose": "用途：register/reset_password", "expires_at": "过期时间",
		"used_at": "使用时间", "created_at": "创建时间",
	},
	"refresh_tokens": {
		"id": "主键（雪花 ID）", "user_id": "商户用户 ID", "token_hash": "刷新令牌哈希",
		"expires_at": "过期时间", "revoked_at": "吊销时间",
		"user_agent": "客户端 UA", "created_at": "创建时间",
	},
}

func syncComments(ctx context.Context, db *sql.DB, dbName string) error {
	for table, comment := range tableComments {
		q := fmt.Sprintf("ALTER TABLE `%s` COMMENT = %s", table, quoteMySQLString(comment))
		if _, err := db.ExecContext(ctx, q); err != nil {
			if isUnknownTable(err) {
				continue
			}
			return fmt.Errorf("table comment %s: %w", table, err)
		}
	}
	for table, cols := range columnComments {
		for col, comment := range cols {
			if err := syncColumnComment(ctx, db, dbName, table, col, comment); err != nil {
				return err
			}
		}
	}
	return nil
}

func syncColumnComment(ctx context.Context, db *sql.DB, dbName, table, column, comment string) error {
	var colType, isNullable, extra string
	var colDefault sql.NullString
	err := db.QueryRowContext(ctx, `
SELECT COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?`,
		dbName, table, column,
	).Scan(&colType, &isNullable, &colDefault, &extra)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("describe %s.%s: %w", table, column, err)
	}

	nullSQL := "NULL"
	if isNullable == "NO" {
		nullSQL = "NOT NULL"
	}
	q := fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s` %s %s", table, column, colType, nullSQL)
	if colDefault.Valid {
		q += " DEFAULT " + quoteMySQLDefault(colDefault.String, colType)
	}
	if extra := strings.TrimSpace(extra); extra != "" {
		q += " " + extra
	}
	q += " COMMENT " + quoteMySQLString(comment)
	if _, err := db.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("column comment %s.%s: %w", table, column, err)
	}
	return nil
}

func quoteMySQLString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func quoteMySQLDefault(v, colType string) string {
	t := strings.ToLower(colType)
	if strings.Contains(t, "int") || strings.Contains(t, "decimal") ||
		strings.Contains(t, "float") || strings.Contains(t, "double") ||
		strings.HasPrefix(t, "bit") {
		if v == "" {
			return "NULL"
		}
		return v
	}
	if strings.EqualFold(v, "CURRENT_TIMESTAMP") || strings.HasPrefix(strings.ToUpper(v), "CURRENT_TIMESTAMP") {
		return v
	}
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

func isUnknownTable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Error 1146") || strings.Contains(msg, "doesn't exist")
}
