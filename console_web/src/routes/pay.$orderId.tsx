import { Link, createFileRoute } from '@tanstack/react-router'
import { CheckCircle2, Clock3, Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { Shell } from '@/components/Shell'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  ApiError,
  alipaySchemaURL,
  api,
  isMobileUA,
  payDisplayURL,
  payPayload,
  type WalletPayOrder,
} from '@/lib/api'
import { shortTime, yuanFromCents } from '@/lib/format'
import { labels } from '@/lib/labels'
import { requireAuth } from '@/lib/requireAuth'
import { cn } from '@/lib/utils'

export const Route = createFileRoute('/pay/$orderId')({
  beforeLoad: () => requireAuth(),
  component: PayPage,
})

function remainSeconds(order: WalletPayOrder | null, nowMs: number): number {
  if (!order) return 0
  if (order.expire_at) {
    const t = new Date(order.expire_at).getTime()
    if (!Number.isNaN(t)) return Math.max(0, Math.floor((t - nowMs) / 1000))
  }
  if (typeof order.expire_in === 'number') return Math.max(0, order.expire_in)
  return 0
}

function formatCountdown(sec: number): string {
  const n = Math.max(0, Math.floor(sec))
  const m = Math.floor(n / 60)
  const s = n % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

function QrUnavailable({
  title,
  subtitle,
}: {
  title: string
  subtitle: string
}) {
  return (
    <div className="mx-auto flex w-full max-w-[260px] flex-col items-center gap-3 rounded-xl border border-destructive/20 bg-destructive/5 px-4 py-8 text-center">
      <div className="flex size-12 items-center justify-center rounded-full bg-destructive/10 text-destructive">
        <Clock3 className="size-6" />
      </div>
      <div className="space-y-1">
        <div className="text-sm font-semibold text-destructive">{title}</div>
        <div className="text-xs text-muted-foreground">{subtitle}</div>
      </div>
    </div>
  )
}

function PayPage() {
  const { orderId } = Route.useParams()
  const [order, setOrder] = useState<WalletPayOrder | null>(null)
  const [error, setError] = useState('')
  const [nowMs, setNowMs] = useState(() => Date.now())

  useEffect(() => {
    let cancelled = false
    let timer: number | undefined

    async function load() {
      try {
        const o = await api.getWalletPay(orderId)
        if (cancelled) return
        setOrder(o)
        setError('')
        setNowMs(Date.now())
        if (o.status === 'pending' && !o.expired) {
          timer = window.setTimeout(() => {
            void load()
          }, 2500)
        }
      } catch (err) {
        if (cancelled) return
        setError(err instanceof ApiError ? err.code : '加载失败')
      }
    }

    void load()
    return () => {
      cancelled = true
      if (timer) window.clearTimeout(timer)
    }
  }, [orderId])

  const pending = order?.status === 'pending' && !order.expired
  const remainSec = remainSeconds(order, nowMs)
  const locallyExpired = Boolean(pending && remainSec <= 0)

  useEffect(() => {
    if (!pending || locallyExpired) return
    const id = window.setInterval(() => setNowMs(Date.now()), 1000)
    return () => window.clearInterval(id)
  }, [pending, locallyExpired, orderId])

  const paid = order?.status === 'paid'
  const effectivePending = pending && !locallyExpired
  const isExpired = Boolean(
    locallyExpired || order?.expired || order?.status === 'expired',
  )
  const isClosed = order?.status === 'closed'
  const payYuan = order ? yuanFromCents(order.pay_amount_cents) : ''
  const alipayHref = useMemo(() => {
    if (!order || !effectivePending || order.channel !== 'alipay' || !isMobileUA()) return ''
    return alipaySchemaURL(payPayload(order))
  }, [order, effectivePending])
  const countdownLabel = formatCountdown(remainSec)
  const countdownUrgent = effectivePending && remainSec > 0 && remainSec <= 60

  return (
    <Shell>
      <div className="mb-5 space-y-1 sm:mb-7">
        <h1 className="font-display text-2xl font-bold tracking-tight sm:text-3xl">支付</h1>
        <p className="text-sm text-muted-foreground">扫码付款后，金额将进入手续费钱包。</p>
      </div>

      {error ? (
        <Card>
          <CardContent className="space-y-3 py-6">
            <div className="text-sm text-destructive">{error}</div>
            <Button asChild variant="outline">
              <Link to="/wallet">返回钱包</Link>
            </Button>
          </CardContent>
        </Card>
      ) : !order ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="size-4 animate-spin" />
          加载订单…
        </div>
      ) : (
        <div className="grid w-full gap-4 lg:grid-cols-[minmax(0,320px)_1fr]">
          <Card>
            <CardHeader className="pb-2">
              <CardDescription>{labels.channel(order.channel)}收款码</CardDescription>
              <CardTitle className="font-display text-3xl tabular-nums">¥ {payYuan}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {effectivePending ? (
                <div
                  className={cn(
                    'inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-sm font-semibold tabular-nums',
                    countdownUrgent
                      ? 'bg-destructive/10 text-destructive'
                      : 'bg-primary/10 text-primary',
                  )}
                >
                  <span className="font-medium text-muted-foreground">剩余</span>
                  {countdownLabel}
                </div>
              ) : null}
              {effectivePending ? (
                <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-3 text-center text-sm font-semibold text-destructive">
                  必须支付指定金额 ¥{payYuan}
                  <div className="mt-1 text-xs font-medium opacity-90">改金额、少付或多付都无法自动到账</div>
                </div>
              ) : null}
              {alipayHref ? (
                <div className="space-y-1.5">
                  <Button asChild className="w-full bg-[#1677ff] hover:bg-[#1668e3]">
                    <a href={alipayHref}>打开支付宝支付</a>
                  </Button>
                  <p className="text-center text-xs text-muted-foreground">
                    跳转支付宝后请确认付款金额为 ¥{payYuan}
                  </p>
                </div>
              ) : null}
              {effectivePending && payDisplayURL(order) ? (
                <img
                  src={payDisplayURL(order)}
                  alt="收款码"
                  className="mx-auto w-full max-w-[260px] rounded-lg border bg-white p-2"
                />
              ) : effectivePending && !alipayHref ? (
                <p className="text-sm text-destructive">暂无收款码</p>
              ) : null}
              {paid ? (
                <div className="flex items-center gap-2 rounded-lg bg-primary/10 px-3 py-3 text-sm text-primary">
                  <CheckCircle2 className="size-4" />
                  已支付，手续费已入账
                </div>
              ) : null}
              {isExpired && !paid ? (
                <>
                  <QrUnavailable title="二维码已过期" subtitle="请返回钱包重新发起充值" />
                  <div className="rounded-lg bg-destructive/10 px-3 py-3 text-sm font-medium text-destructive">
                    订单已过期，扫码将无法到账
                  </div>
                </>
              ) : null}
              {isClosed && !paid ? (
                <>
                  <QrUnavailable title="订单已关闭" subtitle="此付款码已失效" />
                  <div className="rounded-lg bg-destructive/10 px-3 py-3 text-sm font-medium text-destructive">
                    订单已关闭，请重新发起充值
                  </div>
                </>
              ) : null}
              {!effectivePending &&
              !paid &&
              !isExpired &&
              !isClosed ? (
                <div className="text-sm text-muted-foreground">订单状态：{labels.orderStatus(order.status)}</div>
              ) : null}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-lg">订单信息</CardTitle>
              <CardDescription>
                {order.purpose === 'test' ? '在线测试' : '手续费充值'} · {order.out_trade_no}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              <Row label="标价" value={`¥ ${yuanFromCents(order.amount_cents)}`} />
              <Row label="实付" value={`¥ ${payYuan}`} />
              <Row label="渠道" value={labels.channel(order.channel)} />
              <Row
                label="状态"
                value={
                  isExpired ? '已过期' : isClosed ? '已关闭' : labels.orderStatus(order.status)
                }
              />
              <Row
                label="剩余时间"
                value={effectivePending ? countdownLabel : isExpired ? '00:00' : '—'}
                emphasize={effectivePending}
                urgent={countdownUrgent}
              />
              <Row label="过期" value={shortTime(order.expire_at)} />
              {order.paid_at ? <Row label="支付时间" value={shortTime(order.paid_at)} /> : null}
              {effectivePending ? (
                <div className="rounded-lg bg-amber-500/10 px-3 py-2 text-xs font-medium text-amber-800 dark:text-amber-200">
                  重要：付款金额必须与实付金额完全一致，否则无法匹配入账。
                </div>
              ) : null}
              {effectivePending ? (
                <div className="flex items-center gap-2 pt-1 text-muted-foreground">
                  <Loader2 className="size-3.5 animate-spin" />
                  等待支付中，自动刷新…
                </div>
              ) : null}
              <div className="flex flex-wrap gap-2 pt-2">
                <Button asChild variant="outline">
                  <Link to="/wallet">钱包流水</Link>
                </Button>
                <Button asChild variant="outline">
                  <Link to="/">返回总览</Link>
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </Shell>
  )
}

function Row({
  label,
  value,
  emphasize,
  urgent,
}: {
  label: string
  value: string
  emphasize?: boolean
  urgent?: boolean
}) {
  return (
    <div className="flex items-center justify-between gap-3 border-b border-border/60 py-2 last:border-0">
      <span className="text-muted-foreground">{label}</span>
      <span
        className={cn(
          'font-medium tabular-nums',
          emphasize && !urgent && 'text-primary',
          urgent && 'text-destructive',
        )}
      >
        {value}
      </span>
    </div>
  )
}
