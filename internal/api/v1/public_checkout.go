package v1

import (
	"net/http"

	"nairo/api_service/ent"
	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

type PublicCheckoutHandler struct {
	orders *service.OrderService
}

func NewPublicCheckoutHandler(orders *service.OrderService) *PublicCheckoutHandler {
	return &PublicCheckoutHandler{orders: orders}
}

// GetJSON 公开收银台数据（与 PayOrderView 结构一致）。
func (h *PublicCheckoutHandler) GetJSON(c *gin.Context) {
	oid, err := parseID(c.Param("order_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	o, err := h.orders.GetByID(c.Request.Context(), oid)
	if err != nil {
		mapOrderErr(c, err)
		return
	}
	if o.Purpose != "" && o.Purpose != service.PurposeCollect {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	var code *ent.PayCode
	if o.PayCodeID != nil {
		code, _ = h.orders.GetPayCode(c.Request.Context(), *o.PayCodeID)
	}
	c.JSON(http.StatusOK, buildPayOrderView(o, code, payOrderOpts{
		OnlyPendingCode: true,
	}))
}

func (h *PublicCheckoutHandler) Page(c *gin.Context) {
	// 允许商户站点用 iframe 嵌套收银台；不设置 X-Frame-Options。
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Content-Security-Policy", "frame-ancestors *")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Writer.Header().Del("X-Frame-Options")
	c.String(http.StatusOK, checkoutHTML)
}

const checkoutHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover" />
  <meta name="theme-color" content="#f3f5f8" />
  <meta name="robots" content="noindex,nofollow" />
  <title>付款</title>
  <style>
    :root {
      --ink: #1a2332;
      --ink-soft: #334155;
      --muted: #64748b;
      --line: rgba(26, 35, 50, 0.08);
      --panel: #fff;
      --ok: #0f766e;
      --ok-soft: rgba(15, 118, 110, 0.12);
      --danger: #b91c1c;
      --danger-soft: rgba(185, 28, 28, 0.1);
      --ch: #64748b;
      --ch-soft: rgba(100, 116, 139, 0.1);
      --ch-deep: #475569;
      --radius: 20px;
      --font: -apple-system, BlinkMacSystemFont, "PingFang SC", "Hiragino Sans GB",
        "Segoe UI", "Microsoft YaHei", "Noto Sans SC", "Helvetica Neue", Arial, sans-serif;
    }
    body.ch-wechat {
      --ch: #07c160;
      --ch-soft: rgba(7, 193, 96, 0.12);
      --ch-deep: #06ad56;
    }
    body.ch-alipay {
      --ch: #1677ff;
      --ch-soft: rgba(22, 119, 255, 0.12);
      --ch-deep: #0b5fff;
    }
    * { box-sizing: border-box; }
    html, body { min-height: 100%; }
    body {
      margin: 0;
      font-family: var(--font);
      color: var(--ink);
      background:
        radial-gradient(1000px 520px at 0% -10%, color-mix(in srgb, var(--ch) 18%, transparent), transparent 55%),
        radial-gradient(800px 480px at 100% 110%, rgba(15, 118, 110, 0.08), transparent 50%),
        linear-gradient(180deg, #f3f5f8 0%, #eef1f6 100%);
      background-attachment: fixed;
      -webkit-font-smoothing: antialiased;
      text-align: left;
    }
    .shell {
      position: relative;
      z-index: 1;
      width: min(560px, 100%);
      margin: 0 auto;
      padding:
        max(20px, env(safe-area-inset-top))
        max(16px, env(safe-area-inset-right))
        max(28px, env(safe-area-inset-bottom))
        max(16px, env(safe-area-inset-left));
      min-height: 100dvh;
      display: flex;
      flex-direction: column;
      justify-content: flex-start;
    }
    @media (min-width: 768px) {
      .shell {
        padding-top: max(40px, env(safe-area-inset-top));
        justify-content: flex-start;
      }
    }
    .panel {
      width: 100%;
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: var(--radius);
      padding: 0;
      overflow: hidden;
      box-shadow: 0 18px 40px rgba(26, 35, 50, 0.08);
      animation: rise 0.5s cubic-bezier(0.22, 1, 0.36, 1) both;
    }
    .channel-bar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 16px 20px;
      background: var(--ch-soft);
      border-bottom: 1px solid color-mix(in srgb, var(--ch) 16%, transparent);
    }
    .channel-brand {
      display: flex;
      align-items: center;
      gap: 12px;
      min-width: 0;
    }
    .channel-logo {
      width: 40px;
      height: 40px;
      border-radius: 12px;
      display: grid;
      place-items: center;
      color: #fff;
      background: linear-gradient(160deg, color-mix(in srgb, var(--ch) 88%, #fff), var(--ch-deep));
      box-shadow: 0 8px 18px color-mix(in srgb, var(--ch) 28%, transparent);
      flex-shrink: 0;
    }
    .channel-logo svg { width: 22px; height: 22px; display: block; }
    .channel-copy { min-width: 0; }
    .channel-copy strong {
      display: block;
      font-size: 1rem;
      font-weight: 700;
      color: var(--ch-deep);
      line-height: 1.25;
    }
    .channel-copy span {
      display: block;
      margin-top: 2px;
      font-size: 0.8rem;
      color: var(--muted);
      font-weight: 500;
    }
    .status-pill {
      display: inline-flex;
      align-items: center;
      gap: 7px;
      padding: 6px 11px;
      border-radius: 999px;
      font-size: 0.75rem;
      font-weight: 600;
      background: rgba(255,255,255,0.72);
      color: var(--ch-deep);
      white-space: nowrap;
    }
    .status-pill.live::before {
      content: "";
      width: 6px; height: 6px; border-radius: 50%;
      background: currentColor;
      animation: pulse 1.6s ease-in-out infinite;
    }
    .status-pill.ok { color: var(--ok); }
    .status-pill.err { color: var(--danger); }
    .body {
      padding: 22px 20px 22px;
    }
    .layout {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      grid-template-areas: "head" "qr" "act" "meta";
    }
    .head {
      grid-area: head;
      display: flex;
      flex-wrap: wrap;
      align-items: flex-end;
      justify-content: space-between;
      gap: 10px 12px;
    }
    .qr-col { grid-area: qr; margin-top: 12px; }
    .act { grid-area: act; }
    .layout > .meta-line { grid-area: meta; }
    @media (min-width: 560px) {
      .layout {
        grid-template-columns: minmax(0, 1fr) 220px;
        grid-template-areas: "head qr" "act qr" "meta qr";
        grid-template-rows: auto auto 1fr;
        column-gap: 22px;
        align-items: start;
      }
      .qr-col { margin-top: 0; }
      .layout.noqr {
        grid-template-columns: minmax(0, 1fr);
        grid-template-areas: "head" "act" "meta";
      }
    }
    .amount-label {
      font-size: 0.8rem;
      font-weight: 600;
      letter-spacing: 0.06em;
      color: var(--muted);
    }
    .amount {
      margin-top: 2px;
      font-size: clamp(2.2rem, 7vw, 2.75rem);
      font-weight: 600;
      line-height: 1.2;
      color: var(--ink);
    }
    .amount span {
      font-size: 0.55em;
      margin-right: 2px;
      font-weight: 500;
      opacity: 0.7;
      vertical-align: 0.12em;
    }
    .countdown {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      padding: 8px 12px;
      border-radius: 10px;
      background: rgba(26, 35, 50, 0.05);
      color: var(--ink-soft);
      font-size: 0.88rem;
      font-weight: 500;
    }
    .countdown strong {
      color: var(--ch-deep);
      font-weight: 600;
    }
    .countdown.urgent {
      background: var(--danger-soft);
      color: var(--danger);
    }
    .countdown.urgent strong { color: var(--danger); }
    .lock {
      margin-top: 14px;
      padding: 12px 14px;
      border-radius: 12px;
      border: 1px solid rgba(185, 28, 28, 0.16);
      background: var(--danger-soft);
      color: #9f1239;
      font-size: 0.86rem;
      font-weight: 600;
      line-height: 1.45;
    }
    .lock em {
      font-style: normal;
      font-weight: 600;
      color: var(--ink);
    }
    .pay-actions { margin-top: 14px; display: grid; gap: 8px; }
    .btn-pay {
      appearance: none;
      border: 0;
      cursor: pointer;
      width: 100%;
      padding: 13px 16px;
      border-radius: 12px;
      font-family: var(--font);
      font-size: 0.98rem;
      font-weight: 700;
      color: #fff;
      background: linear-gradient(180deg, color-mix(in srgb, var(--ch) 85%, #fff), var(--ch-deep));
      box-shadow: 0 10px 22px color-mix(in srgb, var(--ch) 26%, transparent);
      text-decoration: none;
      text-align: center;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
    }
    .btn-pay svg { width: 18px; height: 18px; }
    .btn-pay:active { transform: translateY(1px); opacity: 0.95; }
    .btn-note {
      font-size: 0.78rem;
      color: var(--muted);
      font-weight: 500;
    }
    .qr-wrap {
      width: min(100%, 240px, 32dvh);
      aspect-ratio: 1;
      margin: 0 auto;
      padding: 12px;
      border-radius: 16px;
      background: linear-gradient(160deg, #fff 0%, #f8fafc 100%);
      border: 1px solid color-mix(in srgb, var(--ch) 18%, var(--line));
      box-shadow: 0 10px 24px rgba(26, 35, 50, 0.06);
      position: relative;
      overflow: hidden;
    }
    @media (min-width: 560px) {
      .qr-wrap {
        width: 220px;
        max-width: 220px;
        margin: 0;
        padding: 14px;
      }
    }
    .qr-wrap::after {
      content: "";
      position: absolute;
      inset: -45%;
      background: linear-gradient(115deg, transparent 40%, rgba(255,255,255,0.55) 50%, transparent 60%);
      animation: sheen 3s ease-in-out infinite;
      pointer-events: none;
    }
    .qr-wrap img {
      width: 100%;
      height: 100%;
      object-fit: contain;
      display: block;
      position: relative;
      z-index: 1;
    }
    .qr-wrap.expired {
      display: flex;
      align-items: center;
      justify-content: center;
      background: #f8fafc;
    }
    .qr-wrap.expired::after { display: none; }
    .qr-expired {
      position: relative;
      z-index: 1;
      display: grid;
      gap: 6px;
      justify-items: start;
      padding: 10px;
      color: var(--danger);
      text-align: left;
    }
    .qr-expired svg { width: 32px; height: 32px; }
    .qr-expired strong { font-size: 0.95rem; font-weight: 700; }
    .qr-expired span { font-size: 0.78rem; font-weight: 500; color: var(--muted); line-height: 1.4; }
    .hint {
      margin: 10px 0 0;
      text-align: center;
      font-size: 0.86rem;
      color: var(--muted);
      font-weight: 500;
      line-height: 1.45;
    }
    .hint b { color: var(--ch-deep); font-weight: 700; }
    @media (min-width: 560px) { .hint { text-align: left; margin-top: 12px; } }
    .banner {
      margin-top: 14px;
      padding: 12px 14px;
      border-radius: 12px;
      font-weight: 600;
      font-size: 0.92rem;
    }
    .banner.ok { background: var(--ok-soft); color: var(--ok); }
    .banner.err { background: var(--danger-soft); color: var(--danger); }
    .meta-line {
      margin-top: 16px;
      padding-top: 14px;
      border-top: 1px solid var(--line);
      font-size: 0.8rem;
      color: var(--muted);
      word-break: break-all;
      line-height: 1.5;
    }
    .meta-line span { color: var(--ink-soft); font-weight: 600; }
    .sk {
      display: block;
      border-radius: 8px;
      background: linear-gradient(90deg, rgba(26,35,50,0.05), rgba(26,35,50,0.11), rgba(26,35,50,0.05));
      background-size: 200% 100%;
      animation: shimmer 1.2s linear infinite;
    }
    .sk-logo { width: 40px; height: 40px; border-radius: 12px; flex-shrink: 0; }
    .sk-pill { width: 72px; height: 26px; border-radius: 999px; }
    .sk-cd { width: 96px; height: 34px; border-radius: 10px; }
    .qr-wrap.sk-qr { padding: 0; box-shadow: none; border-color: transparent; }
    .qr-wrap.sk-qr::after { display: none; }
    .channel-bar.sk-bar { background: rgba(26, 35, 50, 0.03); border-bottom-color: var(--line); }

    @media (max-width: 559px) {
      .shell {
        padding-top: max(12px, env(safe-area-inset-top));
        padding-bottom: max(12px, env(safe-area-inset-bottom));
      }
      .channel-bar { padding: 11px 16px; }
      .channel-logo, .sk-logo { width: 34px; height: 34px; border-radius: 10px; }
      .channel-logo svg { width: 19px; height: 19px; }
      .channel-copy strong { font-size: 0.95rem; }
      .channel-copy span { font-size: 0.75rem; margin-top: 1px; }
      .status-pill { padding: 5px 10px; font-size: 0.72rem; }
      .body { padding: 14px 16px 16px; }
      .amount-label { font-size: 0.74rem; }
      .amount { font-size: clamp(1.9rem, 9vw, 2.4rem); }
      .countdown { padding: 6px 10px; font-size: 0.82rem; }
      .lock { margin-top: 10px; padding: 8px 12px; font-size: 0.78rem; line-height: 1.4; }
      .pay-actions { margin-top: 10px; gap: 6px; }
      .btn-pay { padding: 12px 14px; font-size: 0.95rem; }
      .btn-note { text-align: center; font-size: 0.74rem; }
      .hint { margin-top: 8px; font-size: 0.8rem; line-height: 1.35; }
      .banner { margin-top: 10px; padding: 10px 12px; font-size: 0.88rem; }
      .meta-line { margin-top: 10px; padding-top: 10px; font-size: 0.74rem; line-height: 1.4; }
      .qr-expired { justify-items: center; text-align: center; }
    }
    @keyframes rise {
      from { opacity: 0; transform: translateY(14px); }
      to { opacity: 1; transform: translateY(0); }
    }
    @keyframes pulse {
      0%, 100% { opacity: 1; transform: scale(1); }
      50% { opacity: 0.4; transform: scale(0.85); }
    }
    @keyframes sheen {
      0%, 60% { transform: translateX(-45%); }
      100% { transform: translateX(45%); }
    }
    @keyframes shimmer {
      0% { background-position: 200% 0; }
      100% { background-position: -200% 0; }
    }
    html.is-embed, html.is-embed body {
      background: transparent;
      background-attachment: initial;
      min-height: 0;
    }
    html.is-embed .shell {
      min-height: 0;
      width: min(560px, 100%);
      padding: 8px;
      justify-content: flex-start;
    }
    html.is-embed .panel { box-shadow: 0 8px 24px rgba(26, 35, 50, 0.06); }
    @media (prefers-reduced-motion: reduce) {
      *, *::before, *::after { animation: none !important; transition: none !important; }
    }
  </style>
</head>
<body>
  <main class="shell">
    <section class="panel" id="box" aria-live="polite" aria-busy="true">
      <div class="channel-bar sk-bar">
        <div class="channel-brand">
          <span class="sk sk-logo"></span>
          <div class="channel-copy">
            <span class="sk" style="width:116px;height:15px"></span>
            <span class="sk" style="width:148px;height:11px;margin-top:7px"></span>
          </div>
        </div>
        <span class="sk sk-pill"></span>
      </div>
      <div class="body"><div class="layout">
        <div class="head">
          <div>
            <span class="sk" style="width:56px;height:11px"></span>
            <span class="sk" style="width:150px;height:36px;margin-top:8px"></span>
          </div>
          <span class="sk sk-cd"></span>
        </div>
        <div class="qr-col">
          <div class="qr-wrap sk sk-qr"></div>
          <span class="sk" style="width:60%;height:12px;margin:12px auto 0"></span>
        </div>
        <div class="act"><span class="sk" style="height:44px;border-radius:12px;margin-top:12px"></span></div>
        <div class="meta-line"><span class="sk" style="width:70%;height:12px"></span></div>
      </div></div>
    </section>
  </main>
  <script>
    const orderId = location.pathname.split('/').filter(Boolean).pop();
    const params = new URLSearchParams(location.search);
    const forceEmbed = params.get('embed') === '1' || params.get('embed') === 'true';
    const inFrame = forceEmbed || (function () {
      try { return window.self !== window.top; } catch (_) { return true; }
    })();
    if (inFrame) document.documentElement.classList.add('is-embed');
    const parentOrigin = params.get('parent_origin') || '*';
    const box = document.getElementById('box');
    const yuan = (c) => (Number(c || 0) / 100).toFixed(2);
    const stMap = { pending: '待支付', paid: '已支付', expired: '已过期', closed: '已关闭' };
    // Simple Icons (CC0)
    const ICON_WECHAT = '<svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M8.691 2.188C3.891 2.188 0 5.476 0 9.53c0 2.212 1.17 4.203 3.002 5.55a.59.59 0 0 1 .213.665l-.39 1.48c-.019.07-.048.141-.048.213 0 .163.13.295.29.295a.326.326 0 0 0 .167-.054l1.903-1.114a.864.864 0 0 1 .717-.098 10.16 10.16 0 0 0 2.837.403c.276 0 .543-.027.811-.05-.857-2.578.157-4.972 1.932-6.446 1.703-1.415 3.882-1.98 5.853-1.838-.576-3.583-4.196-6.348-8.596-6.348zM5.785 5.991c.642 0 1.162.529 1.162 1.18a1.17 1.17 0 0 1-1.162 1.178A1.17 1.17 0 0 1 4.623 7.17c0-.651.52-1.18 1.162-1.18zm5.813 0c.642 0 1.162.529 1.162 1.18a1.17 1.17 0 0 1-1.162 1.178 1.17 1.17 0 0 1-1.162-1.178c0-.651.52-1.18 1.162-1.18zm5.34 2.867c-1.797-.052-3.746.512-5.28 1.786-1.72 1.428-2.687 3.72-1.78 6.22.942 2.453 3.666 4.229 6.884 4.229.826 0 1.622-.12 2.361-.336a.722.722 0 0 1 .598.082l1.584.926a.272.272 0 0 0 .14.047c.134 0 .24-.111.24-.247 0-.06-.023-.12-.038-.177l-.327-1.233a.582.582 0 0 1-.023-.156.49.49 0 0 1 .201-.398C23.024 18.48 24 16.82 24 14.98c0-3.21-2.931-5.837-6.656-6.088V8.89c-.135-.01-.27-.027-.407-.03zm-2.53 3.274c.535 0 .969.44.969.982a.976.976 0 0 1-.969.983.976.976 0 0 1-.969-.983c0-.542.434-.982.97-.982zm4.844 0c.535 0 .969.44.969.982a.976.976 0 0 1-.969.983.976.976 0 0 1-.969-.983c0-.542.434-.982.969-.982z"/></svg>';
    const ICON_ALIPAY = '<svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M19.695 15.07c3.426 1.158 4.203 1.22 4.203 1.22V3.846c0-2.124-1.705-3.845-3.81-3.845H3.914C1.808.001.102 1.722.102 3.846v16.31c0 2.123 1.706 3.845 3.813 3.845h16.173c2.105 0 3.81-1.722 3.81-3.845v-.157s-6.19-2.602-9.315-4.119c-2.096 2.602-4.8 4.181-7.607 4.181-4.75 0-6.361-4.19-4.112-6.949.49-.602 1.324-1.175 2.617-1.497 2.025-.502 5.247.313 8.266 1.317a16.796 16.796 0 0 0 1.341-3.302H5.781v-.952h4.799V6.975H4.77v-.953h5.81V3.591s0-.409.411-.409h2.347v2.84h5.744v.951h-5.744v1.704h4.69a19.453 19.453 0 0 1-1.986 5.06c1.424.52 2.702 1.011 3.654 1.333m-13.81-2.032c-.596.06-1.71.325-2.321.869-1.83 1.608-.735 4.55 2.968 4.55 2.151 0 4.301-1.388 5.99-3.61-2.403-1.182-4.438-2.028-6.637-1.809"/></svg>';
    let expireAtMs = 0;
    let redirectTimer = 0;
    let tickTimer = 0;
    let pollTimer = 0;
    let lastData = null;
    let lastPostedKey = '';

    function channelMeta(ch) {
      if (ch === 'wechat') {
        return {
          id: 'wechat',
          name: '微信',
          title: '请使用微信支付',
          tip: '打开微信扫一扫完成付款',
          btn: '打开微信付款',
          icon: ICON_WECHAT,
        };
      }
      return {
        id: 'alipay',
        name: '支付宝',
        title: '请使用支付宝支付',
        tip: '打开支付宝扫一扫完成付款',
        btn: '打开支付宝付款',
        icon: ICON_ALIPAY,
      };
    }
    function applyChannelTheme(ch) {
      document.body.classList.remove('ch-wechat', 'ch-alipay');
      document.body.classList.add(ch === 'wechat' ? 'ch-wechat' : 'ch-alipay');
      const meta = document.querySelector('meta[name="theme-color"]');
      if (meta) meta.setAttribute('content', ch === 'wechat' ? '#e8f8ef' : '#e8f1ff');
    }
    // 仅允许 http/https，返回规范化后的绝对地址；非法返回空串
    function safeReturnURL(u) {
      try {
        const x = new URL(String(u || ''));
        return (x.protocol === 'http:' || x.protocol === 'https:') ? x.href : '';
      } catch (_) { return ''; }
    }
    // 支付成功且订单带 return_url：倒计时后跳转（iframe 嵌入时不跳，交给商家页面处理）
    function scheduleReturn(d) {
      const url = safeReturnURL(d.return_url);
      const link = document.getElementById('rl');
      if (!url || !link) return;
      link.href = url;
      if (inFrame || redirectTimer) return;
      let left = 3;
      const step = function () {
        const el = document.getElementById('rt');
        if (el) el.textContent = String(left);
        if (left <= 0) { location.href = url; return; }
        left -= 1;
        redirectTimer = setTimeout(step, 1000);
      };
      step();
    }
    function postToParent(type, payload) {
      if (!inFrame || !window.parent || window.parent === window) return;
      const msg = Object.assign({
        source: 'nairo-checkout',
        type: type,
        order_id: orderId,
      }, payload || {});
      try { window.parent.postMessage(msg, parentOrigin); }
      catch (_) { try { window.parent.postMessage(msg, '*'); } catch (__) {} }
    }
    function notifyStatus(d) {
      if (!d) return;
      const status = (d.expired && d.status === 'pending') ? 'expired' : d.status;
      const key = status + '|' + (d.out_trade_no || '') + '|' + (d.pay_amount_cents || 0);
      if (key === lastPostedKey) return;
      lastPostedKey = key;
      postToParent('checkout.status', {
        out_trade_no: d.out_trade_no || '',
        status: status,
        expired: !!d.expired,
        pay_amount_cents: d.pay_amount_cents || 0,
        channel: d.channel || '',
        return_url: safeReturnURL(d.return_url),
      });
    }
    function qrSrc(d) {
      if (d.pay_code && d.pay_code.display_url) return d.pay_code.display_url;
      return d.qr_url || '';
    }
    function payPayload(d) {
      if (d.pay_code && d.pay_code.payload) return String(d.pay_code.payload).trim();
      if (d.qr_content) return String(d.qr_content).trim();
      const u = qrSrc(d);
      try {
        const url = new URL(u, location.origin);
        const data = url.searchParams.get('data');
        if (data) return data.trim();
      } catch (_) {}
      return '';
    }
    function isMobile() {
      return /Android|iPhone|iPad|iPod|Mobile|HarmonyOS/i.test(navigator.userAgent || '');
    }
    function alipaySchemaURL(payload) {
      const raw = String(payload || '').trim();
      if (!raw) return '';
      if (/^alipays?:\/\//i.test(raw) || /^alipayqr:\/\//i.test(raw)) return raw;
      let qrcode = raw;
      if (!/^https?:\/\//i.test(qrcode)) {
        if (/qr\.alipay\.com/i.test(qrcode)) qrcode = 'https://' + qrcode.replace(/^\/\//, '');
        else if (/^https%3A/i.test(qrcode) || /^http%3A/i.test(qrcode)) {
          try { qrcode = decodeURIComponent(qrcode); } catch (_) {}
        }
      }
      return 'alipays://platformapi/startapp?saId=10000007&qrcode=' + encodeURIComponent(qrcode);
    }
    function remainSeconds() {
      if (!expireAtMs) return 0;
      return Math.max(0, Math.floor((expireAtMs - Date.now()) / 1000));
    }
    function fmtCountdown(sec) {
      const n = Math.max(0, Math.floor(Number(sec) || 0));
      if (n <= 0) return '00:00';
      const m = Math.floor(n / 60);
      const s = n % 60;
      return String(m).padStart(2, '0') + ':' + String(s).padStart(2, '0');
    }
    function updateCountdown() {
      const el = document.getElementById('cd');
      if (!el || !lastData) return;
      const pending = lastData.status === 'pending' && !lastData.expired;
      if (!pending) {
        el.textContent = '00:00';
        el.parentElement && el.parentElement.classList.remove('urgent');
        return;
      }
      const sec = remainSeconds();
      el.textContent = fmtCountdown(sec);
      if (sec > 0 && sec <= 60) el.parentElement && el.parentElement.classList.add('urgent');
      else el.parentElement && el.parentElement.classList.remove('urgent');
      if (sec <= 0 && lastData.status === 'pending') {
        lastData = Object.assign({}, lastData, { expired: true, status: 'expired' });
        render(lastData);
      }
    }
    function startTick() {
      if (tickTimer) clearInterval(tickTimer);
      updateCountdown();
      tickTimer = setInterval(updateCountdown, 1000);
    }
    function unavailableQrBlock(title, sub, hint) {
      return ''
        + '<div class="qr-wrap expired" aria-label="' + title + '">'
        +   '<div class="qr-expired">'
        +     '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">'
        +       '<circle cx="12" cy="12" r="9"></circle>'
        +       '<path d="M12 7v5l3 2"></path>'
        +     '</svg>'
        +     '<strong>' + title + '</strong>'
        +     '<span>' + sub + '</span>'
        +   '</div>'
        + '</div>'
        + '<p class="hint">' + hint + '</p>';
    }

    async function load() {
      try {
        const res = await fetch('/api/v1/public/checkout/' + encodeURIComponent(orderId));
        const data = await res.json();
        if (!res.ok) throw new Error(data.message || data.error || '加载失败');
        lastData = data;
        if (data.expire_at) expireAtMs = new Date(data.expire_at).getTime();
        else if (typeof data.expire_in === 'number') expireAtMs = Date.now() + data.expire_in * 1000;
        render(data);
        notifyStatus(data);
        if (pollTimer) clearTimeout(pollTimer);
        if (data.status === 'pending' && !data.expired) {
          pollTimer = setTimeout(load, 2500);
          startTick();
        } else if (tickTimer) {
          clearInterval(tickTimer);
          tickTimer = 0;
        }
      } catch (e) {
        box.innerHTML = '<div class="body"><div class="banner err">' + (e.message || '加载失败') + '</div></div>';
        postToParent('checkout.error', { message: e.message || '加载失败' });
      }
    }

    function render(d) {
      const paid = d.status === 'paid';
      const closed = d.status === 'closed';
      const timedOut = expireAtMs > 0 && remainSeconds() <= 0 && d.status === 'pending';
      const expired = !!d.expired || d.status === 'expired' || timedOut;
      const pending = d.status === 'pending' && !expired;
      const failed = expired || closed;
      const ch = channelMeta(d.channel);
      applyChannelTheme(ch.id);
      const qr = qrSrc(d);
      const payload = payPayload(d);
      const showQr = pending && qr;
      const alipayLink = (pending && ch.id === 'alipay' && isMobile()) ? alipaySchemaURL(payload) : '';
      const payYuan = yuan(d.pay_amount_cents);
      const pillClass = paid ? 'ok' : failed ? 'err' : 'live';
      const pillText = paid ? '支付成功' : closed ? '已关闭' : expired ? '已过期' : '等待付款';
      const channelTip = paid ? '付款已完成' : closed ? '订单已关闭' : expired ? '订单已过期，请重新下单' : ch.tip;

      let head = '<div><div class="amount-label">应付金额</div>';
      head += '<div class="amount"><span>¥</span>' + payYuan + '</div></div>';
      if (pending) {
        head += '<div class="countdown"><span>剩余</span><strong id="cd">' + fmtCountdown(remainSeconds() || d.expire_in) + '</strong></div>';
      }

      let act = '';
      if (pending) {
        act += '<div class="lock">请支付指定金额 <em>¥' + payYuan + '</em>，多付少付均无法到账</div>';
      }
      if (alipayLink) {
        act += '<div class="pay-actions">';
        act += '<a class="btn-pay" href="' + alipayLink + '">' + ch.icon + ch.btn + '</a>';
        act += '<div class="btn-note">跳转后请确认金额为 ¥' + payYuan + '</div>';
        act += '</div>';
      }
      if (paid) {
        act += '<div class="banner ok">付款成功</div>';
        if (safeReturnURL(d.return_url)) {
          act += '<div class="pay-actions"><a class="btn-pay" id="rl" href="#"><span>立即返回商家' + (inFrame ? '' : '（<span id="rt">3</span>s）') + '</span></a></div>';
        }
      }
      else if (closed) act += '<div class="banner err">订单已关闭</div>';
      else if (expired) act += '<div class="banner err">订单已过期，请重新下单</div>';
      let meta = '';
      if (d.out_trade_no || d.order_id) {
        meta = '<div class="meta-line">单号 <span>' + (d.out_trade_no || d.order_id) + '</span></div>';
      }

      let qrCol = '';
      if (showQr) {
        qrCol += '<div class="qr-wrap"><img alt="付款二维码" src="' + qr + '" /></div>';
        qrCol += '<p class="hint">' + (alipayLink ? '也可截图后用<b>' + ch.name + '</b>扫码' : '请使用<b>' + ch.name + '</b>扫一扫完成付款') + '</p>';
      } else if (pending && !qr && !alipayLink) {
        qrCol += '<div class="banner err">暂无收款码，请稍后重试或联系商家</div>';
      } else if (closed) {
        qrCol += unavailableQrBlock('订单已关闭', '此付款码已失效', '如需支付请联系商家重新下单');
      } else if (expired && !paid) {
        qrCol += unavailableQrBlock('二维码已过期', '请返回商家页面重新下单', '支付时效已结束，扫码将无法到账');
      }

      let html = '';
      html += '<div class="channel-bar">';
      html +=   '<div class="channel-brand">';
      html +=     '<div class="channel-logo">' + ch.icon + '</div>';
      html +=     '<div class="channel-copy"><strong>' + ch.title + '</strong><span>' + channelTip + '</span></div>';
      html +=   '</div>';
      html +=   '<span class="status-pill ' + pillClass + '">' + pillText + '</span>';
      html += '</div>';
      html += '<div class="body"><div class="layout' + (qrCol ? '' : ' noqr') + '">';
      html +=   '<div class="head">' + head + '</div>';
      if (qrCol) html += '<div class="qr-col">' + qrCol + '</div>';
      if (act) html += '<div class="act">' + act + '</div>';
      html +=   meta;
      html += '</div></div>';

      box.setAttribute('aria-busy', 'false');
      box.innerHTML = html;
      if (pending) updateCountdown();
      if (paid) scheduleReturn(d);
      notifyStatus(d);
    }
    postToParent('checkout.ready', {});
    load();
  </script>
</body>
</html>
`