package v1

import (
	"time"

	"nairo/api_service/ent"
)

// PayCodeView 前后端统一的收款码结构。
// 展示二维码请只用 display_url 作为 <img src>；image_url / payload 为库内原始字段。
type PayCodeView struct {
	ID         string `json:"id"`
	Channel    string `json:"channel,omitempty"`
	CodeType   string `json:"code_type"`
	ImageURL   string `json:"image_url"`
	Payload    string `json:"payload"`
	DisplayURL string `json:"display_url"`
}

// PayOrderView 收银台 / 充值 / 演示 / Open API 共用订单展示结构。
type PayOrderView struct {
	OrderID           string       `json:"order_id"`
	OutTradeNo        string       `json:"out_trade_no"`
	Purpose           string       `json:"purpose,omitempty"`
	AmountCents       int          `json:"amount_cents"`
	PayAmountCents    int          `json:"pay_amount_cents"`
	AdjustOffsetCents int          `json:"adjust_offset_cents"`
	Channel           string       `json:"channel"`
	Status            string       `json:"status"`
	ExpireAt          *time.Time   `json:"expire_at,omitempty"`
	ExpireIn          int64        `json:"expire_in,omitempty"`
	Expired           bool         `json:"expired,omitempty"`
	PaidAt            *time.Time   `json:"paid_at,omitempty"`
	CreatedAt         *time.Time   `json:"created_at,omitempty"`
	PayPage           string       `json:"pay_page,omitempty"`
	PayURL            string       `json:"pay_url,omitempty"`
	AccountID         string       `json:"account_id,omitempty"`
	SettleType        string       `json:"settle_type,omitempty"`
	ReturnURL         string       `json:"return_url,omitempty"`
	PayCode           *PayCodeView `json:"pay_code,omitempty"`
	// 兼容旧字段：与 pay_code.display_url / payload 同值
	QRURL     string `json:"qr_url,omitempty"`
	QRContent string `json:"qr_content,omitempty"`
	Warning   string `json:"warning,omitempty"`
}

type payOrderOpts struct {
	PublicBase string
	PayPage    string
	PayURL     string
	AccountID  string
	Warning    string
	// AttachCodeWhenPending：仅 pending 且未过期时附带收款码
	OnlyPendingCode bool
}

func buildPayOrderView(o *ent.Order, code *ent.PayCode, opts payOrderOpts) PayOrderView {
	view := PayOrderView{
		OrderID:           formatInt64(o.ID),
		OutTradeNo:        o.OutTradeNo,
		Purpose:           o.Purpose,
		AmountCents:       o.AmountCents,
		PayAmountCents:    o.PayAmountCents,
		AdjustOffsetCents: o.AdjustOffsetCents,
		Channel:           o.Channel,
		Status:            o.Status,
		PaidAt:            o.PaidAt,
		PayPage:           opts.PayPage,
		PayURL:            opts.PayURL,
		AccountID:         opts.AccountID,
		SettleType:        o.SettleType,
		ReturnURL:         o.ReturnURL,
		Warning:           opts.Warning,
	}
	created := o.CreatedAt
	if !created.IsZero() {
		view.CreatedAt = &created
	}
	fillExpire(&view, o.ExpireAt, o.Status)

	attach := code != nil
	if opts.OnlyPendingCode {
		attach = code != nil && o.Status == "pending" && !view.Expired
	}
	if attach {
		attachPayCode(&view, code, opts.PublicBase)
	}
	return view
}

func viewPayCode(code *ent.PayCode, publicBase string) *PayCodeView {
	if code == nil {
		return nil
	}
	payload := ""
	if code.Payload != nil {
		payload = *code.Payload
	}
	display, content := resolveQRURL(code.ImageURL, code.Payload)
	if content == "" {
		content = payload
	}
	return &PayCodeView{
		ID:         formatInt64(code.ID),
		Channel:    code.Channel,
		CodeType:   code.CodeType,
		ImageURL:   code.ImageURL,
		Payload:    content,
		DisplayURL: absolutizeURL(publicBase, display),
	}
}

func attachPayCode(view *PayOrderView, code *ent.PayCode, publicBase string) {
	pc := viewPayCode(code, publicBase)
	if pc == nil {
		return
	}
	view.PayCode = pc
	view.QRURL = pc.DisplayURL
	view.QRContent = pc.Payload
}

func fillExpire(view *PayOrderView, expireAt *time.Time, status string) {
	view.ExpireAt = expireAt
	if expireAt == nil {
		return
	}
	sec := int64(time.Until(*expireAt).Seconds())
	if sec < 0 {
		sec = 0
		if status == "pending" {
			view.Expired = true
		}
	}
	view.ExpireIn = sec
}
