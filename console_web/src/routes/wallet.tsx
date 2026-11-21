import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { ChannelPicker } from '@/components/ChannelPicker'
import { Shell } from '@/components/Shell'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ApiError, api, type LedgerItem, type ProfileResult } from '@/lib/api'
import { feeLabel, shortTime, yuanFromCents } from '@/lib/format'
import { labels } from '@/lib/labels'
import { requireAuth } from '@/lib/requireAuth'

export const Route = createFileRoute('/wallet')({
  beforeLoad: () => requireAuth(),
  component: WalletPage,
})

function WalletPage() {
  const navigate = useNavigate()
  const [profile, setProfile] = useState<ProfileResult | null>(null)
  const [ledgers, setLedgers] = useState<LedgerItem[]>([])
  const [configured, setConfigured] = useState(false)
  const [isPayUser, setIsPayUser] = useState(false)
  const [amountYuan, setAmountYuan] = useState('50')
  const [channel, setChannel] = useState('wechat')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    void Promise.all([api.me(), api.ledgers(), api.walletPayStatus()])
      .then(([p, l, st]) => {
        setProfile(p)
        setLedgers(l.items)
        setConfigured(st.configured)
        setIsPayUser(Boolean(st.is_platform_pay_user))
      })
      .catch(() => toast.error('加载失败'))
  }, [])

  async function startRecharge() {
    if (isPayUser) {
      toast.error('当前是平台收款商户，不能给自己充值；请用管理后台入账')
      return
    }
    const yuan = Number(amountYuan)
    if (!Number.isFinite(yuan) || yuan <= 0) {
      toast.error('请输入有效金额')
      return
    }
    setBusy(true)
    try {
      const res = await api.createWalletPay({
        amount_cents: Math.round(yuan * 100),
        channel,
        purpose: 'recharge',
      })
      void navigate({ to: '/pay/$orderId', params: { orderId: res.order_id } })
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === 'platform_pay_user_self') {
          toast.error('当前是平台收款商户，不能给自己充值')
        } else if (err.code.includes('platform_pay_user')) {
          toast.error('平台尚未配置收款商户')
        } else if (err.code === 'no_channel_device') {
          toast.error(err.detail || '未开启该支付通道设备，请先在设备中开启')
        } else if (err.code === 'no_pay_code' || err.code === 'channel_busy') {
          toast.error(err.detail || '暂无收款码，请上传同金额固额码或任意金额码')
        } else {
          toast.error(err.detail || err.code)
        }
      } else {
        toast.error('创建失败')
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <Shell>
      <div className="mb-5 flex flex-col gap-3 sm:mb-7 sm:flex-row sm:items-end sm:justify-between">
        <div className="space-y-1">
          <h1 className="font-display text-2xl font-bold tracking-tight sm:text-3xl">钱包流水</h1>
          <p className="text-sm text-muted-foreground">手续费余额、在线充值与流水。</p>
        </div>
        <Button asChild variant="outline">
          <Link to="/developers">管理 API 凭证</Link>
        </Button>
      </div>

      <div className="mb-4 grid gap-3 lg:grid-cols-2">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>手续费余额</CardDescription>
            <CardTitle className="font-display text-3xl tabular-nums">
              ¥ {yuanFromCents(profile?.wallet.balance_cents)}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground">
            费率 {feeLabel(profile?.user.fee_rate_bps)}（只读）
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-lg">在线充值</CardTitle>
            <CardDescription>扫码支付后按标价入账手续费钱包</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {!configured ? (
              <p className="text-sm text-muted-foreground">平台尚未配置收款商户，暂无法在线充值。</p>
            ) : isPayUser ? (
              <p className="text-sm text-muted-foreground">
                当前账号是平台收款商户，不能扫自己的码充值。请在管理后台给该商户入账，或换其他商户账号。
              </p>
            ) : (
              <>
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
                <Button type="button" disabled={busy} onClick={() => void startRecharge()}>
                  {busy ? <Loader2 className="size-4 animate-spin" /> : null}
                  去支付
                </Button>
              </>
            )}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-lg">流水</CardTitle>
          <CardDescription>最近 50 条</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {ledgers.length === 0 ? (
            <div className="py-8 text-center text-sm text-muted-foreground">暂无流水</div>
          ) : (
            ledgers.map((l) => (
              <div
                key={l.id}
                className="flex flex-col gap-1 rounded-xl border p-3 sm:flex-row sm:items-center sm:justify-between"
              >
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge variant="outline">{labels.ledgerType(l.type)}</Badge>
                    <span className="text-xs text-muted-foreground">{shortTime(l.created_at)}</span>
                  </div>
                  {l.remark ? <div className="text-sm text-muted-foreground">{l.remark}</div> : null}
                </div>
                <div className="text-right">
                  <div
                    className={`font-medium tabular-nums ${l.amount_cents < 0 ? 'text-destructive' : 'text-primary'}`}
                  >
                    {l.amount_cents >= 0 ? '+' : ''}
                    ¥ {yuanFromCents(l.amount_cents)}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    余额 ¥ {yuanFromCents(l.balance_after_cents)}
                  </div>
                </div>
              </div>
            ))
          )}
        </CardContent>
      </Card>
    </Shell>
  )
}
