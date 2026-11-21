import { createFileRoute } from '@tanstack/react-router'
import { Copy, RefreshCw } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { Shell } from '@/components/Shell'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ApiError, api } from '@/lib/api'
import { requireAuth } from '@/lib/requireAuth'
import { useConfirm } from '@/components/ConfirmDialog'

export const Route = createFileRoute('/developers')({
  beforeLoad: () => requireAuth(),
  component: DevelopersPage,
})

function CredentialRow({
  label,
  hint,
  value,
  onCopy,
}: {
  label: string
  hint: string
  value: string
  onCopy: () => void
}) {
  return (
    <div className="rounded-xl border p-3">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between sm:gap-3">
        <div className="min-w-0 flex-1">
          <div className="text-sm font-medium text-foreground">{label}</div>
          <code className="mt-1 block break-all font-mono text-sm text-foreground">
            {value || '—'}
          </code>
        </div>
        {value ? (
          <Button size="sm" variant="outline" className="h-10 w-full shrink-0 sm:h-8 sm:w-auto" onClick={onCopy}>
            <Copy className="size-4" />
            复制
          </Button>
        ) : null}
      </div>
      <p className="mt-2 text-xs leading-relaxed text-muted-foreground">{hint}</p>
    </div>
  )
}

function FieldTable({
  rows,
}: {
  rows: Array<{ name: string; type: string; required?: string; desc: string }>
}) {
  return (
    <div className="overflow-x-auto rounded-xl border">
      <table className="w-full min-w-[520px] text-left text-sm">
        <thead className="bg-muted/50 text-xs text-muted-foreground">
          <tr>
            <th className="px-3 py-2 font-medium">字段</th>
            <th className="px-3 py-2 font-medium">类型</th>
            <th className="px-3 py-2 font-medium">说明</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.name} className="border-t align-top">
              <td className="px-3 py-2">
                <code className="font-mono text-xs">{r.name}</code>
                {r.required ? (
                  <div className="mt-0.5 text-[11px] text-muted-foreground">{r.required}</div>
                ) : null}
              </td>
              <td className="px-3 py-2 text-xs text-muted-foreground">{r.type}</td>
              <td className="px-3 py-2 text-muted-foreground">{r.desc}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function DevelopersPage() {
  const confirm = useConfirm()
  const [merchantId, setMerchantId] = useState('')
  const [accessKey, setAccessKey] = useState('')
  const [secretOnce, setSecretOnce] = useState('')
  const [busy, setBusy] = useState(false)
  const origin = useMemo(() => (typeof window !== 'undefined' ? window.location.origin : ''), [])

  useEffect(() => {
    void api
      .credential()
      .then((c) => {
        setMerchantId(c.merchant_id)
        setAccessKey(c.access_key)
      })
      .catch(() => toast.error('加载凭证失败'))
  }, [])

  async function resetSecret() {
    const ok = await confirm({
      title: '重置 API Secret',
      description: '确认重置 API Secret？商户 ID 与 Access Key 保持不变，旧 Secret 立即失效。',
      confirmText: '重置',
      destructive: true,
    })
    if (!ok) return
    setBusy(true)
    try {
      const res = await api.resetCredential()
      setMerchantId(res.merchant_id)
      setAccessKey(res.access_key)
      setSecretOnce(res.api_secret)
      toast.success('已重置 Secret，请立即保存')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.code : '重置失败')
    } finally {
      setBusy(false)
    }
  }

  function copyText(text: string, tip = '已复制') {
    void navigator.clipboard.writeText(text).then(
      () => toast.success(tip),
      () => toast.error('复制失败'),
    )
  }

  function copyPair() {
    copyText(`merchant_id=${merchantId}\naccess_key=${accessKey}`, '商户 ID + Key 已复制')
  }

  const createReqSample = `{
  "out_trade_no": "demo-1001",
  "amount_cents": 1000,
  "channel": "wechat",
  "notify_url": "https://your.app/notify",
  "return_url": "https://your.app/orders/demo-1001"
}`

  const createResSample = `{
  "order_id": "1234567890123456789",
  "out_trade_no": "demo-1001",
  "amount_cents": 1000,
  "pay_amount_cents": 1001,
  "adjust_offset_cents": 1,
  "channel": "wechat",
  "status": "pending",
  "account_id": "…",
  "expire_at": "2026-11-18T18:05:00+08:00",
  "expire_in": 300,
  "pay_url": "${origin || 'https://api.example.com'}/checkout/1234567890123456789",
  "qr_url": "${origin || 'https://api.example.com'}/api/v1/public/qr.png?data=…",
  "qr_content": "wxp://…",
  "pay_code": {
    "id": "…",
    "channel": "wechat",
    "code_type": "float",
    "image_url": "",
    "payload": "wxp://…",
    "display_url": "${origin || 'https://api.example.com'}/api/v1/public/qr.png?data=…"
  }
}`

  const queryResSample = `{
  "order_id": "1234567890123456789",
  "out_trade_no": "demo-1001",
  "amount_cents": 1000,
  "pay_amount_cents": 1001,
  "adjust_offset_cents": 1,
  "channel": "wechat",
  "status": "paid",
  "settle_type": "auto",
  "paid_at": "2026-11-18T18:01:22+08:00",
  "expire_at": "2026-11-18T18:05:00+08:00",
  "expire_in": 0,
  "created_at": "2026-11-18T18:00:00+08:00",
  "pay_url": "${origin || 'https://api.example.com'}/checkout/1234567890123456789",
  "pay_code": {
    "id": "…",
    "channel": "wechat",
    "code_type": "float",
    "image_url": "",
    "payload": "wxp://…",
    "display_url": "${origin || 'https://api.example.com'}/api/v1/public/qr.png?data=…"
  }
}`

  return (
    <Shell>
      <div className="mb-5 space-y-1 sm:mb-7">
        <h1 className="font-display text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
          开发者 API
        </h1>
        <p className="text-sm text-muted-foreground">
          下单直接返回收银台 URL、收款二维码与失效时间；查询可轮询支付状态。
        </p>
      </div>

      <div className="mb-4 grid items-stretch gap-3 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-lg text-foreground">商户凭证</CardTitle>
            <CardDescription>业务系统配置这两项即可识别商户；Secret 单独保管</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <Button
              size="sm"
              variant="outline"
              className="h-10 w-full sm:h-8 sm:w-auto"
              disabled={!merchantId || !accessKey}
              onClick={copyPair}
            >
              <Copy className="size-4" />
              复制 ID + Key
            </Button>
            <CredentialRow
              label="商户 ID"
              hint="当前登录商户的唯一编号，对接时写入业务配置即可"
              value={merchantId}
              onCopy={() => copyText(merchantId, '商户 ID 已复制')}
            />
            <CredentialRow
              label="Access Key（商户 Key）"
              hint="放在请求头 X-Nairo-Key，用于识别商户"
              value={accessKey}
              onCopy={() => copyText(accessKey, 'Access Key 已复制')}
            />
            <div className="rounded-xl border border-dashed p-3">
              <div className="text-sm font-medium text-foreground">API Secret</div>
              <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
                用于请求签名，仅在重置时显示一次，请妥善保存。
              </p>
              <Button
                className="mt-3 h-10 w-full sm:h-8 sm:w-auto"
                variant="destructive"
                size="sm"
                disabled={busy}
                onClick={() => void resetSecret()}
              >
                <RefreshCw className="size-4" />
                重置 API Secret
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-lg text-foreground">签名约定</CardTitle>
            <CardDescription>Base {origin}/api/v1/open</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <pre className="overflow-x-auto rounded-xl border bg-muted/50 p-4 font-mono text-[11px] leading-relaxed text-foreground">{`# 用 API Secret 对下方字符串做签名后放入请求头
待签名内容 =
  请求方法 + 换行 +
  请求路径 + 换行 +
  时间戳(秒) + 换行 +
  请求体摘要

请求头：
  X-Nairo-Key: ${accessKey || '<Access Key>'}
  X-Nairo-Timestamp: <unix 秒>
  X-Nairo-Sign: <签名>
  Content-Type: application/json

查询订单（GET）时请求体按空字符串计算摘要。`}</pre>
            <div className="flex flex-wrap gap-2">
              <Badge variant="secondary">时间戳有效约 5 分钟</Badge>
              <Badge variant="secondary">商户订单号不可重复</Badge>
            </div>
          </CardContent>
        </Card>
      </div>

      {secretOnce ? (
        <Alert className="mb-4 border-primary/30 bg-primary/5">
          <AlertTitle className="text-foreground">新 API Secret（仅显示一次）</AlertTitle>
          <AlertDescription className="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center">
            <code className="break-all rounded border bg-background px-2 py-1 font-mono text-sm text-foreground">
              {secretOnce}
            </code>
            <Button size="sm" variant="outline" onClick={() => copyText(secretOnce)}>
              复制
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setSecretOnce('')}>
              我已保存
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      <Card className="mb-4">
        <CardHeader>
          <CardTitle className="text-lg text-foreground">POST /api/v1/open/orders · 创建订单</CardTitle>
          <CardDescription>
            分配收款码并锁定实付金额。可直接使用返回的 pay_url 或 pay_code.display_url 展示给用户。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div>
            <div className="mb-2 text-sm font-medium">请求 Body</div>
            <FieldTable
              rows={[
                { name: 'out_trade_no', type: 'string', required: '必填', desc: '商户侧订单号，同一商户下唯一' },
                { name: 'amount_cents', type: 'int', required: '必填', desc: '标价金额，单位分，例如 1000 = ¥10.00' },
                { name: 'channel', type: 'string', required: '必填', desc: 'wechat 或 alipay' },
                { name: 'notify_url', type: 'string', required: '选填', desc: '支付成功后异步回调 URL（POST JSON）' },
                { name: 'return_url', type: 'string', required: '选填', desc: '支付成功后收银台页面自动跳转的地址（http/https，最长 512）；iframe 嵌入时不跳转，改由 postMessage 通知' },
              ]}
            />
            <pre className="mt-3 overflow-x-auto rounded-xl border bg-muted/50 p-4 font-mono text-[11px] leading-relaxed">
              {createReqSample}
            </pre>
            <Button className="mt-2" size="sm" variant="outline" onClick={() => copyText(createReqSample)}>
              <Copy className="size-4" />
              复制请求示例
            </Button>
          </div>

          <div>
            <div className="mb-2 text-sm font-medium">成功响应 200</div>
            <FieldTable
              rows={[
                { name: 'order_id', type: 'string', desc: '平台订单号' },
                { name: 'out_trade_no', type: 'string', desc: '商户订单号（回显）' },
                { name: 'amount_cents', type: 'int', desc: '标价（分）' },
                { name: 'pay_amount_cents', type: 'int', desc: '实付金额（分），付款人须按此金额支付' },
                { name: 'adjust_offset_cents', type: 'int', desc: '实付相对标价的差额（分）' },
                { name: 'channel', type: 'string', desc: 'wechat / alipay' },
                { name: 'status', type: 'string', desc: 'pending 待支付' },
                { name: 'expire_at', type: 'datetime', desc: '订单失效时间' },
                { name: 'expire_in', type: 'int', desc: '距失效剩余秒数' },
                { name: 'pay_url', type: 'string', desc: '收银台页面地址，可跳转打开' },
                { name: 'pay_code', type: 'object', desc: '收款码；展示用 pay_code.display_url' },
                { name: 'pay_code.display_url', type: 'string', desc: '可直接作为 <img src> 的二维码地址' },
                { name: 'pay_code.payload', type: 'string', desc: '原始码内容（链接等）' },
                { name: 'qr_url', type: 'string', desc: '兼容字段，等同 pay_code.display_url' },
                { name: 'account_id', type: 'string', desc: '收款设备编号' },
              ]}
            />
            <pre className="mt-3 overflow-x-auto rounded-xl border bg-muted/50 p-4 font-mono text-[11px] leading-relaxed">
              {createResSample}
            </pre>
            <Button className="mt-2" size="sm" variant="outline" onClick={() => copyText(createResSample)}>
              <Copy className="size-4" />
              复制响应示例
            </Button>
          </div>

          <div className="rounded-xl border bg-muted/30 p-3 text-sm text-muted-foreground">
            <div className="font-medium text-foreground">接入建议</div>
            <ul className="mt-2 list-disc space-y-1 pl-5">
              <li>
                优先跳转 <code className="font-mono text-xs">pay_url</code>，或把{' '}
                <code className="font-mono text-xs">pay_code.display_url</code> 做成图片展示
              </li>
              <li>
                页面/文案务必写清 <code className="font-mono text-xs">pay_amount_cents</code> 实付金额
              </li>
              <li>
                用 <code className="font-mono text-xs">expire_at</code> /{' '}
                <code className="font-mono text-xs">expire_in</code> 做倒计时，过期后重新下单
              </li>
            </ul>
          </div>

          <div>
            <div className="mb-2 text-sm font-medium">常见错误</div>
            <FieldTable
              rows={[
                { name: '402 insufficient_balance', type: 'error', desc: '手续费钱包余额不足' },
                { name: '409 duplicate_out_trade_no', type: 'error', desc: 'out_trade_no 重复' },
                { name: '409 no_channel_device', type: 'error', desc: '商户没有开启该支付通道的监控设备' },
                { name: '409 no_pay_code', type: 'error', desc: '暂无收款码（优先同金额固额码，否则任意金额码）' },
                { name: '409 channel_busy', type: 'error', desc: '渠道繁忙，请稍后重试' },
                { name: '400 invalid', type: 'error', desc: '参数不合法（渠道或金额等）' },
              ]}
            />
          </div>
        </CardContent>
      </Card>

      <Card className="mb-4">
        <CardHeader>
          <CardTitle className="text-lg text-foreground">
            GET /api/v1/open/orders/:out_trade_no · 查询订单
          </CardTitle>
          <CardDescription>按商户订单号查询状态；支付中可轮询，建议间隔 2–3 秒。</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div>
            <div className="mb-2 text-sm font-medium">路径参数</div>
            <FieldTable
              rows={[
                {
                  name: 'out_trade_no',
                  type: 'string',
                  required: '必填',
                  desc: '创建时传入的商户订单号（需 URL Encode）',
                },
              ]}
            />
          </div>
          <div>
            <div className="mb-2 text-sm font-medium">成功响应 200</div>
            <FieldTable
              rows={[
                { name: 'order_id', type: 'string', desc: '平台订单号' },
                { name: 'out_trade_no', type: 'string', desc: '商户订单号' },
                { name: 'amount_cents / pay_amount_cents', type: 'int', desc: '标价 / 实付（分）' },
                {
                  name: 'status',
                  type: 'string',
                  desc: 'pending 待支付 / paid 已支付 / expired 已过期 / closed 已关闭',
                },
                { name: 'settle_type', type: 'string', desc: 'auto 自动入账 / manual 人工入账' },
                { name: 'paid_at', type: 'datetime|null', desc: '支付完成时间' },
                { name: 'expire_at', type: 'datetime', desc: '失效时间' },
                { name: 'expire_in', type: 'int', desc: '剩余秒数（已过期为 0）' },
                { name: 'pay_url', type: 'string', desc: '收银台地址' },
                { name: 'pay_code', type: 'object', desc: '收款码；展示用 pay_code.display_url' },
                { name: 'created_at', type: 'datetime', desc: '创建时间' },
              ]}
            />
            <pre className="mt-3 overflow-x-auto rounded-xl border bg-muted/50 p-4 font-mono text-[11px] leading-relaxed">
              {queryResSample}
            </pre>
            <Button className="mt-2" size="sm" variant="outline" onClick={() => copyText(queryResSample)}>
              <Copy className="size-4" />
              复制响应示例
            </Button>
          </div>
          <div className="rounded-xl border bg-muted/30 p-3 text-sm text-muted-foreground">
            支付成功后若配置了 <code className="font-mono text-xs">notify_url</code>
            ，平台会向该地址发送支付结果通知；业务侧仍建议再调查询接口确认状态。
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-lg text-foreground">收银台页面</CardTitle>
          <CardDescription>下单返回的 pay_url，可跳转或 iframe 嵌套</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3 text-sm text-muted-foreground">
          <p>
            地址形如：
            <code className="font-mono text-xs text-foreground">{origin}/checkout/订单号</code>
          </p>
          <p>页面展示实付金额与收款码，并自动刷新支付状态；过期后请重新下单。已允许跨站 iframe 嵌套。</p>
          <pre className="overflow-x-auto rounded-lg border bg-muted/40 p-3 font-mono text-xs text-foreground">{`<iframe
  src="${origin}/checkout/订单号?embed=1"
  title="付款"
  style="width:100%;min-height:560px;border:0;border-radius:16px;"
  allow="payment *"
></iframe>`}</pre>
          <p>
            嵌套时页面会向父窗口 <code className="font-mono text-xs">postMessage</code>
            ：
            <code className="font-mono text-xs">checkout.ready</code> /
            <code className="font-mono text-xs">checkout.status</code> /
            <code className="font-mono text-xs">checkout.error</code>
            。可用 <code className="font-mono text-xs">?parent_origin=https://你的站点</code>{' '}
            限制消息目标；不传则向 <code className="font-mono text-xs">*</code> 广播。
          </p>
          <pre className="overflow-x-auto rounded-lg border bg-muted/40 p-3 font-mono text-xs text-foreground">{`window.addEventListener('message', (e) => {
  const d = e.data;
  if (!d || d.source !== 'nairo-checkout') return;
  if (d.type === 'checkout.status' && d.status === 'paid') {
    // 付款成功：跳转结果页 / 刷新订单（d.return_url 为下单时传入的 return_url）
  }
});`}</pre>
        </CardContent>
      </Card>
    </Shell>
  )
}
