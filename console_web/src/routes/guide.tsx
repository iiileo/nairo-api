import { Link, createFileRoute } from '@tanstack/react-router'
import { CheckCircle2, Smartphone, Upload, WalletCards } from 'lucide-react'
import { Shell } from '@/components/Shell'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { requireAuth } from '@/lib/requireAuth'

export const Route = createFileRoute('/guide')({
  beforeLoad: () => requireAuth(),
  component: GuidePage,
})

const steps = [
  {
    icon: WalletCards,
    title: '1. 确认手续费余额',
    body: '平台按费率从预充余额扣费。余额不足时无法下单，请联系运营充值。',
    to: '/wallet',
    cta: '查看钱包',
  },
  {
    icon: Smartphone,
    title: '2. 添加监控设备并扫码绑定 App',
    body: '创建设备、选择微信/支付宝渠道，用奈洛 App 扫码绑定；开启通知使用权与保活。',
    to: '/accounts',
    cta: '去管设备',
  },
  {
    icon: Upload,
    title: '3. 上传对应渠道收款码',
    body: '按微信 / 支付宝分别添加任意金额码（或固额码）。下单时会按渠道匹配收款码。',
    to: '/accounts',
    cta: '管理收款码',
  },
  {
    icon: CheckCircle2,
    title: '4. 业务系统调 Open API 下单',
    body: '签名创建订单后，直接使用返回的 pay_url（收银台）或 pay_code.display_url（二维码图片），并按 expire_at / expire_in 做倒计时；到账后自动匹配。字段说明见「开发者 API」。',
    to: '/developers',
    cta: '查看开发者 API',
  },
]

function GuidePage() {
  return (
    <Shell>
      <div className="mb-5 space-y-1 sm:mb-7">
        <h1 className="font-display text-2xl font-bold tracking-tight sm:text-3xl">使用教程</h1>
        <p className="text-sm text-muted-foreground">从开户到收款的最短路径；异常可在订单 / 未匹配到账人工处理。</p>
      </div>

      <div className="mb-6 grid gap-3 lg:grid-cols-2">
        {steps.map((s) => {
          const Icon = s.icon
          return (
            <Card key={s.title} className="overflow-hidden">
              <CardHeader className="space-y-3">
                <div className="flex size-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
                  <Icon className="size-5" />
                </div>
                <div>
                  <CardTitle className="text-lg">{s.title}</CardTitle>
                  <CardDescription className="mt-1.5 text-sm leading-relaxed">{s.body}</CardDescription>
                </div>
              </CardHeader>
              <CardContent>
                <Button asChild size="sm" variant="outline">
                  <Link to={s.to}>{s.cta}</Link>
                </Button>
              </CardContent>
            </Card>
          )
        })}
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-lg">日常验收清单</CardTitle>
          <CardDescription>上线前建议逐项确认</CardDescription>
        </CardHeader>
        <CardContent>
          <ul className="space-y-2 text-sm leading-relaxed text-muted-foreground">
            <li>· App 显示在线，通知使用权与电池优化已开启</li>
            <li>· 微信、支付宝各至少一张「任意金额」收款码</li>
            <li>· 金额微调开关与步长符合业务预期</li>
            <li>· 用 Open API 下一笔小额测试单，付款后状态变为已支付</li>
            <li>· 故意付错金额时，能在「未匹配到账」看到 orphan 并可补单</li>
          </ul>
          <div className="mt-4 flex flex-wrap gap-2">
            <Button asChild size="sm">
              <Link to="/orders">查看订单</Link>
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link to="/orphans">未匹配到账</Link>
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link to="/developers">开发者 API</Link>
            </Button>
          </div>
        </CardContent>
      </Card>
    </Shell>
  )
}
