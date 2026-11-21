import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Shell } from '@/components/Shell'
import { ApiError, api } from '@/lib/api'
import { labels } from '@/lib/labels'
import { requireAuth } from '@/lib/requireAuth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

export const Route = createFileRoute('/settings')({
  beforeLoad: () => requireAuth(),
  component: SettingsPage,
})

function SettingsPage() {
  const [enabled, setEnabled] = useState(true)
  const [direction, setDirection] = useState('inc')
  const [step, setStep] = useState('1')
  const [maxOffset, setMaxOffset] = useState('50')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    void api.me().then((p) => {
      setEnabled(p.user.adjust_enabled)
      setDirection(p.user.adjust_direction || 'inc')
      setStep(String(p.user.adjust_step_cents ?? 1))
      setMaxOffset(String(p.user.adjust_max_offset_cents ?? 50))
    }).catch(() => toast.error('加载设置失败'))
  }, [])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    const stepCents = Number(step)
    const maxOffsetCents = Number(maxOffset)
    if (!Number.isFinite(stepCents) || stepCents < 1) {
      toast.error('步长至少 1 分')
      return
    }
    if (!Number.isFinite(maxOffsetCents) || maxOffsetCents < 0) {
      toast.error('最大偏移不能为负')
      return
    }
    setBusy(true)
    try {
      const res = await api.updateAdjust({
        adjust_enabled: enabled,
        adjust_direction: direction,
        adjust_step_cents: stepCents,
        adjust_max_offset_cents: maxOffsetCents,
      })
      setEnabled(res.adjust_enabled)
      setDirection(res.adjust_direction)
      setStep(String(res.adjust_step_cents))
      setMaxOffset(String(res.adjust_max_offset_cents))
      toast.success('已保存')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.code : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Shell>
      <div className="mb-5 space-y-1 sm:mb-7">
        <h1 className="font-display text-2xl font-bold tracking-tight sm:text-3xl">金额微调</h1>
        <p className="text-sm text-muted-foreground">
          并发下单时通过微调实付金额占额；费率由平台设定，不可自改。
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>微调参数</CardTitle>
          <CardDescription>
            当前：{enabled ? '开启' : '关闭'} · {labels.adjustDirection(direction)}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form className="space-y-5" onSubmit={onSubmit}>
            <div className="flex items-center justify-between gap-3 rounded-lg border px-3 py-3">
              <div>
                <div className="font-medium">启用微调</div>
                <div className="text-sm text-muted-foreground">关闭后新订单按标价占额</div>
              </div>
              <Switch checked={enabled} onCheckedChange={setEnabled} />
            </div>
            <div className="space-y-2">
              <Label>方向</Label>
              <Select value={direction} onValueChange={(v) => setDirection(v || 'inc')}>
                <SelectTrigger className="h-11 w-full sm:h-9">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="inc">递增</SelectItem>
                  <SelectItem value="dec">递减</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="step">步长（分）</Label>
                <Input id="step" inputMode="numeric" value={step} onChange={(e) => setStep(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="max">最大偏移（分）</Label>
                <Input
                  id="max"
                  inputMode="numeric"
                  value={maxOffset}
                  onChange={(e) => setMaxOffset(e.target.value)}
                />
              </div>
            </div>
            <Button type="submit" disabled={busy} className="w-full sm:w-auto">
              {busy ? '保存中…' : '保存设置'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </Shell>
  )
}
