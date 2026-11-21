import { createFileRoute, Link } from '@tanstack/react-router'
import { BookOpen, Code2, Loader2, MonitorSmartphone, Settings2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { ChannelPicker } from '@/components/ChannelPicker'
import { Shell } from '@/components/Shell'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ApiError, api, type DashboardResult } from '@/lib/api'
import { feeLabel, yuanFromCents } from '@/lib/format'
import { labels } from '@/lib/labels'
import { requireAuth } from '@/lib/requireAuth'

export const Route = createFileRoute('/')({
  beforeLoad: () => requireAuth(),
  component: OverviewPage,
})

function OverviewPage() {
  const [data, setData] = useState<DashboardResult | null>(null)
  const [error, setError] = useState('')
  const [amountYuan, setAmountYuan] = useState('0.10')
  const [channel, setChannel] = useState('alipay')
  const [busy, setBusy] = useState(false)

  // 跳转收银台后用浏览器「返回」会从缓存还原本页（bfcache），此时 busy 仍为 true，按钮会一直转圈
  useEffect(() => {
    const onShow = (e: PageTransitionEvent) => {
      if (e.persisted) setBusy(false)
    }
    window.addEventListener('pageshow', onShow)
    return () => window.removeEventListener('pageshow', onShow)
  }, [])

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const d = await api.dashboard()
        if (cancelled) return
        setData(d)
      } catch {
        if (!cancelled) setError('加载失败')
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  async function startTestPay() {
    const yuan = Number(amountYuan)
    if (!Number.isFinite(yuan) || yuan <= 0) {
      toast.error('请输入有效金额')
      return
    }
    const cents = Math.round(yuan * 100)
    if (cents < 10) {
      toast.error('金额至少 ¥0.10')
      return
    }
    setBusy(true)
    try {
      const res = await api.createTestOrder({ amount_cents: cents, channel })
      const url = res.pay_url || `/checkout/${res.order_id}`
      window.location.assign(url)
      // 兜底：若 10 秒后仍停留在本页（跳转被拦截/很慢），恢复按钮，避免一直 loading
      window.setTimeout(() => setBusy(false), 10000)
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === 'insufficient_balance' || err.code === 'insufficient') {
          toast.error('手续费余额不足，请先充值')
        } else if (err.code === 'no_channel_device') {
          toast.error(err.detail || '未开启该支付通道设备，请先在设备中开启')
        } else if (err.code === 'no_pay_code' || err.code === 'channel_busy') {
          toast.error(err.detail || '暂无该渠道收款码，请先上传')
        } else {
          toast.error(err.detail || err.code)
        }
      } else {
        toast.error('创建失败')
      }
      setBusy(false)
    }
  }

  return (
    <Shell>
      <div className="mb-5 space-y-1 sm:mb-7">
        <h1 className="font-display text-2xl font-bold tracking-tight sm:text-3xl">总览</h1>
        <p className="text-sm text-muted-foreground sm:text-base">
          余额、今日成交与在线设备。可用下方「测试支付」验证自己的收款码与到账。
        </p>
      </div>

      {error ? (
        <div className="mb-4 rounded-lg bg-destructive/10 px-3 py-2 text-sm font-medium text-destructive">
          {error}
        </div>
      ) : null}

      <div className="mb-4 grid gap-3 sm:mb-6 sm:grid-cols-2 lg:grid-cols-4">
        <Card className="overflow-hidden">
          <CardHeader className="pb-2">
            <CardDescription>手续费余额</CardDescription>
            <CardTitle className="font-display text-2xl tabular-nums sm:text-3xl">
              ¥ {yuanFromCents(data?.wallet.balance_cents)}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>今日成交</CardDescription>
            <CardTitle className="font-display text-2xl tabular-nums sm:text-3xl">
              {data?.today_paid.count ?? 0} 笔
            </CardTitle>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground">
            ¥ {yuanFromCents(data?.today_paid.amount_cents)}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>App 在线设备</CardDescription>
            <CardTitle className="font-display text-2xl tabular-nums sm:text-3xl">
              {data?.online_accounts ?? 0}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground">近 35 秒有心跳</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>费率</CardDescription>
            <CardTitle className="font-display text-2xl sm:text-3xl">
              {feeLabel(data?.user.fee_rate_bps)}
            </CardTitle>
          </CardHeader>
        </Card>
      </div>

      <div className="mb-4 grid gap-3 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardDescription>测试支付</CardDescription>
            <CardTitle className="text-lg">下单并打开收银台</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-sm text-muted-foreground">
              使用你自己的微信 / 支付宝收款码创建真实订单，跳转 checkout
              验证监控到账。金额不退回，成功后会按费率扣手续费。
            </p>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-2">
                <Label>金额（元）</Label>
                <Input
                  inputMode="decimal"
                  value={amountYuan}
                  onChange={(e) => setAmountYuan(e.target.value)}
                />
              </div>
              <div className="space-y-2 sm:col-span-2">
                <Label>渠道</Label>
                <ChannelPicker value={channel} onChange={setChannel} />
              </div>
            </div>
            <Button type="button" disabled={busy} onClick={() => void startTestPay()}>
              {busy ? <Loader2 className="size-4 animate-spin" /> : null}
              创建并打开收银台
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardDescription>金额微调</CardDescription>
            <CardTitle className="text-lg">
              {data?.user.adjust_enabled ? '已开启' : '已关闭'} ·{' '}
              {labels.adjustDirection(data?.user.adjust_direction)}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm text-muted-foreground">
            <div>
              步长 {data?.user.adjust_step_cents ?? '—'} 分 · 最大偏移{' '}
              {data?.user.adjust_max_offset_cents ?? '—'} 分
            </div>
            <Button asChild variant="outline" size="sm">
              <Link to="/settings">
                <Settings2 className="size-4" />
                去设置
              </Link>
            </Button>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardDescription>快捷入口</CardDescription>
          <CardTitle className="text-lg">按工作流组织</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
          <Button asChild className="h-11 justify-start sm:h-9">
            <Link to="/accounts">
              <MonitorSmartphone className="size-4" />
              监控设备 / 收款码
            </Link>
          </Button>
          <Button asChild variant="outline" className="h-11 justify-start sm:h-9">
            <Link to="/wallet">在线充值手续费</Link>
          </Button>
          <Button asChild variant="outline" className="h-11 justify-start sm:h-9">
            <Link to="/developers">
              <Code2 className="size-4" />
              开发者 API
            </Link>
          </Button>
          <Button asChild variant="outline" className="h-11 justify-start sm:h-9">
            <Link to="/guide">
              <BookOpen className="size-4" />
              使用教程
            </Link>
          </Button>
          <Button asChild variant="outline" className="h-11 justify-start sm:h-9">
            <Link to="/orders">订单</Link>
          </Button>
          <Button asChild variant="outline" className="h-11 justify-start sm:h-9">
            <Link to="/orphans">未匹配到账</Link>
          </Button>
        </CardContent>
      </Card>
    </Shell>
  )
}
