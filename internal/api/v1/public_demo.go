package v1

import (
	"net/http"

	"nairo/api_service/internal/service"

	"github.com/gin-gonic/gin"
)

type PublicDemoHandler struct {
	recharge *service.RechargeService
}

func NewPublicDemoHandler(recharge *service.RechargeService) *PublicDemoHandler {
	return &PublicDemoHandler{recharge: recharge}
}

func (h *PublicDemoHandler) Status(c *gin.Context) {
	configured, warning, err := h.recharge.PublicDemoStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"configured": configured,
		"min_cents":  service.MinRechargeCents,
		"max_cents":  service.MaxRechargeCents,
		"warning":    warning,
	})
}

type publicDemoReq struct {
	AmountCents int    `json:"amount_cents" binding:"required"`
	Channel     string `json:"channel" binding:"required"`
}

func (h *PublicDemoHandler) Create(c *gin.Context) {
	var req publicDemoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	res, err := h.recharge.CreatePublicDemo(c.Request.Context(), req.AmountCents, req.Channel)
	if err != nil {
		mapOrderErr(c, err)
		return
	}
	view := buildPayOrderView(res.Order, res.PayCode, payOrderOpts{
		PayPage: res.PayPagePath,
		Warning: "演示支付金额不会退回。",
	})
	c.JSON(http.StatusOK, view)
}

func (h *PublicDemoHandler) Get(c *gin.Context) {
	oid, err := parseID(c.Param("order_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}
	o, code, err := h.recharge.GetPublicDemo(c.Request.Context(), oid)
	if err != nil {
		mapConsoleErr(c, err)
		return
	}
	view := buildPayOrderView(o, code, payOrderOpts{
		Warning:         "演示支付金额不会退回。",
		OnlyPendingCode: true,
	})
	c.JSON(http.StatusOK, view)
}

func (h *PublicDemoHandler) Page(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, demoPayHTML)
}

const demoPayHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Nairo 演示支付</title>
  <link rel="stylesheet" href="/assets/style.css?v=20261119f" />
</head>
<body>
  <header>
    <strong>Nairo</strong>
    <nav>
      <a href="/">首页</a>
      <a href="/console/">商户登录</a>
    </nav>
  </header>
  <main class="demo-pay">
    <h1>演示支付</h1>
    <p class="muted">真实扫码到账验证，金额不退回、不入账。用于确认监控链路是否正常。</p>
    <div id="box" class="panel">加载中…</div>
  </main>
  <script>
(function () {
  var id = location.pathname.split('/').pop();
  var box = document.getElementById('box');
  function yuan(c) { return (Number(c || 0) / 100).toFixed(2); }
  function channelLabel(c) { return c === 'alipay' ? '支付宝' : '微信'; }
  function qrSrc(d) {
    if (d.pay_code && d.pay_code.display_url) return d.pay_code.display_url;
    return d.qr_url || '';
  }
  function render(d) {
    if (d.status === 'paid') {
      box.innerHTML = '<div class="ok">已到账</div>' +
        '<p>实付 ¥' + yuan(d.pay_amount_cents) + ' · ' + channelLabel(d.channel) + '</p>' +
        '<p class="muted">单号 ' + d.out_trade_no + '</p>' +
        '<p><a href="/">返回首页</a></p>';
      return false;
    }
    if (d.expired || d.status !== 'pending') {
      box.innerHTML = '<div class="err">订单已失效</div><p><a href="/">重新发起</a></p>';
      return false;
    }
    var qr = qrSrc(d);
    box.innerHTML =
      '<div class="amount">¥ ' + yuan(d.pay_amount_cents) + '</div>' +
      '<div class="muted">' + channelLabel(d.channel) + ' · 请扫码支付</div>' +
      (qr ? '<img class="qr" src="' + qr + '" alt="收款码" />' : '<p class="err">暂无收款码</p>') +
      '<p class="muted">剩余约 ' + (d.expire_in || 0) + ' 秒 · 支付后自动刷新</p>' +
      '<p class="warn">' + (d.warning || '') + '</p>';
    return true;
  }
  function tick() {
    fetch('/api/v1/public/demo-pay/' + id).then(function (r) { return r.json().then(function (d) { return { ok: r.ok, d: d }; }); })
      .then(function (x) {
        if (!x.ok) { box.innerHTML = '<div class="err">' + (x.d.message || x.d.error || '加载失败') + '</div>'; return; }
        if (render(x.d)) setTimeout(tick, 2500);
      }).catch(function () { box.innerHTML = '<div class="err">网络错误</div>'; });
  }
  tick();
})();
  </script>
</body>
</html>
`