import { Link, createFileRoute } from '@tanstack/react-router'
import { Copy, Plus, QrCode, RefreshCw, ScanLine, Trash2 } from 'lucide-react'
import QRCode from 'qrcode'
import { useEffect, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Shell } from '@/components/Shell'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ApiError, api, type AccountItem, type AppBindConfig } from '@/lib/api'
import { CHANNEL_OPTS, accountChannels } from '@/lib/channels'
import { shortTime } from '@/lib/format'
import { labels } from '@/lib/labels'
import { requireAuth } from '@/lib/requireAuth'
import { cn } from '@/lib/utils'
import { useConfirm } from '@/components/ConfirmDialog'

export const Route = createFileRoute('/accounts')({
  beforeLoad: () => requireAuth(),
  component: AccountsPage,
})

function qrPayloadOf(b: AppBindConfig) {
  return {
    v: b.v,
    name: b.name,
    base_url: b.base_url,
    push_url: b.push_url,
    heartbeat_url: b.heartbeat_url,
    callback_id: b.callback_id,
    secret: b.secret,
    channels: b.channels,
  }
}

function AccountsPage() {
  const confirm = useConfirm()
  const [items, setItems] = useState<AccountItem[]>([])
  const [createOpen, setCreateOpen] = useState(false)
  const [bind, setBind] = useState<AppBindConfig | null>(null)
  const [qrDataURL, setQrDataURL] = useState('')
  const [picked, setPicked] = useState<string[]>(['wechat', 'alipay'])
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)

  async function load() {
    try {
      const res = await api.accounts()
      setItems(res.items)
    } catch {
      toast.error('加载账户失败')
    }
  }

  useEffect(() => {
    void load()
  }, [])

  useEffect(() => {
    if (!bind) {
      setQrDataURL('')
      return
    }
    // 二维码只放扫码必需字段：heartbeat_ws_url 由 App 根据 heartbeat_url 推导，去掉后内容更少、版本更低，识别更快
    const raw = JSON.stringify(qrPayloadOf(bind))
    void QRCode.toDataURL(raw, {
      errorCorrectionLevel: 'L',
      margin: 2,
      width: 560,
      color: { dark: '#0f172a', light: '#ffffff' },
    }).then(setQrDataURL, () => toast.error('二维码生成失败'))
  }, [bind])

  function toggleChannel(id: string) {
    setPicked((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]))
  }

  async function saveChannels(acc: AccountItem, next: string[]) {
    if (next.length === 0) {
      toast.error('请至少保留一个支付渠道')
      return
    }
    setBusy(true)
    try {
      const res = await api.updateChannels(acc.id, next)
      toast.success('渠道已更新')
      setItems((prev) =>
        prev.map((a) =>
          a.id === acc.id
            ? {
                ...a,
                channel: res.channel,
                channels: res.channels,
                channel_label: res.channel_label,
              }
            : a,
        ),
      )
    } catch (err) {
      toast.error(err instanceof ApiError ? err.code : '更新渠道失败')
    } finally {
      setBusy(false)
    }
  }

  function toggleAccountChannel(acc: AccountItem, id: string) {
    const cur = accountChannels(acc)
    const next = cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id]
    void saveChannels(acc, next)
  }

  async function createAccount(e: FormEvent) {
    e.preventDefault()
    if (picked.length === 0) {
      toast.error('请至少选择一个支付渠道')
      return
    }
    setBusy(true)
    try {
      const res = await api.createAccount({ channels: picked, name: name.trim() || undefined })
      setBind(res.bind)
      toast.success('设备已创建，请用 App 扫码添加')
      setCreateOpen(false)
      setName('')
      await load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.code : '创建失败')
    } finally {
      setBusy(false)
    }
  }

  // 不重置：用当前密钥重新生成扫码配置
  async function showBindQR(acc: AccountItem) {
    setBusy(true)
    try {
      const res = await api.getBindConfig(acc.id)
      setBind(res.bind)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.code : '获取扫码配置失败')
    } finally {
      setBusy(false)
    }
  }

  async function resetSecret(acc: AccountItem) {
    const ok = await confirm({
      title: '重置密钥',
      description: `确认重置「${acc.name || labels.channel(acc.channel)}」的密钥？旧 App 将立即失效。`,
      confirmText: '重置',
      destructive: true,
    })
    if (!ok) return
    setBusy(true)
    try {
      const res = await api.resetAppSecret(acc.id)
      setBind(res.bind)
      toast.success('已重置，请重新扫码')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.code : '重置失败')
    } finally {
      setBusy(false)
    }
  }

  async function releasePhone(acc: AccountItem) {
    const ok = await confirm({
      title: '解除手机绑定',
      description: `确认解除「${acc.name || labels.channel(acc.channel)}」的手机绑定？原手机将停止上报，之后可用其他手机重新扫码绑定。`,
      confirmText: '解除绑定',
      destructive: true,
    })
    if (!ok) return
    setBusy(true)
    try {
      await api.releasePhone(acc.id)
      toast.success('已解除手机绑定')
      await load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.code : '操作失败')
    } finally {
      setBusy(false)
    }
  }

  async function deleteAccount(acc: AccountItem) {
    const ok = await confirm({
      title: '删除监控设备',
      description: `确认删除监控设备「${acc.name || labels.channel(acc.channel)}」？设备将停用，旧 App 立即失效，历史订单仍保留。`,
      confirmText: '删除',
      destructive: true,
    })
    if (!ok) return
    setBusy(true)
    try {
      await api.deleteAccount(acc.id)
      toast.success('设备已删除')
      await load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.code : '删除失败')
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


  return (
    <Shell>
      <div className="mb-5 flex flex-col gap-3 sm:mb-7 sm:flex-row sm:items-end sm:justify-between">
        <div className="space-y-1">
          <h1 className="font-display text-2xl font-bold tracking-tight sm:text-3xl">监控设备</h1>
          <p className="text-sm text-muted-foreground">
            管理收款监听设备与渠道；收款码请进入对应设备页面维护。
          </p>
        </div>
        <Button
          className="h-11 w-full sm:h-9 sm:w-auto"
          onClick={() => {
            setPicked(['wechat', 'alipay'])
            setCreateOpen(true)
          }}
        >
          <Plus className="size-4" />
          添加设备
        </Button>
      </div>

      <div className="grid gap-3 lg:grid-cols-2">
        {items.length === 0 ? (
          <Card className="lg:col-span-2">
            <CardContent className="py-12 text-center text-muted-foreground">暂无监控设备</CardContent>
          </Card>
        ) : (
          items.map((a) => (
            <Card key={a.id}>
              <CardHeader className="space-y-2">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <CardTitle className="text-lg">
                      {a.name || a.channel_label || labels.channel(a.channel)}
                    </CardTitle>
                    <CardDescription>
                      {a.channel_label || labels.channels(a.channels) || labels.channel(a.channel)}
                    </CardDescription>
                  </div>
                  <div className="flex flex-col items-end gap-1">
                    <Badge variant={a.online ? 'default' : 'secondary'}>
                      {a.online ? 'App 在线' : 'App 离线'}
                    </Badge>
                    <Badge variant="outline">{labels.accountStatus(a.status)}</Badge>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-3 text-sm">
                <div className="space-y-2">
                  <div className="text-muted-foreground">支持渠道（可随时修改）</div>
                  <div className="grid grid-cols-2 gap-2">
                    {CHANNEL_OPTS.map((opt) => {
                      const on = accountChannels(a).includes(opt.id)
                      return (
                        <button
                          key={opt.id}
                          type="button"
                          disabled={busy}
                          onClick={() => toggleAccountChannel(a, opt.id)}
                          className={cn(
                            'rounded-lg border px-3 py-2 text-left text-sm font-semibold transition-colors disabled:opacity-60',
                            on
                              ? 'border-primary bg-primary/10 text-foreground'
                              : 'border-border text-muted-foreground hover:bg-muted/60',
                          )}
                        >
                          <div>{opt.label}</div>
                          <div className="mt-0.5 text-xs font-normal opacity-80">
                            {on ? '已启用' : '未启用'}
                          </div>
                        </button>
                      )
                    })}
                  </div>
                </div>
                <div>
                  <div className="text-muted-foreground">推送 URL</div>
                  <div className="mt-1 break-all font-mono text-xs">{a.callback_url}</div>
                </div>
                <div className="flex flex-wrap items-center gap-2 text-xs">
                  <Badge variant={a.phone_bound ? 'secondary' : 'outline'}>
                    {a.phone_bound
                      ? `已绑定手机${a.phone_name ? `：${a.phone_name}` : ''}`
                      : '未绑定手机'}
                  </Badge>
                  {a.phone_bound ? (
                    <button
                      type="button"
                      disabled={busy}
                      className="text-destructive underline-offset-2 hover:underline disabled:opacity-60"
                      onClick={() => void releasePhone(a)}
                    >
                      解除手机绑定
                    </button>
                  ) : null}
                </div>
                <div className="text-xs text-muted-foreground">
                  最近心跳：{shortTime(a.app_last_seen_at)}
                  {a.app_conn_type
                    ? ` · ${a.app_conn_type === 'ws' || a.app_conn_type === 'websocket' ? 'WebSocket' : a.app_conn_type}`
                    : ''}
                </div>
                <div className="grid grid-cols-2 gap-2 sm:flex sm:flex-wrap">
                  <Button asChild size="sm" className="h-10 sm:h-8">
                    <Link to="/accounts/$accountId" params={{ accountId: a.id }}>
                      <QrCode className="size-4" />
                      收款码
                    </Link>
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="h-10 sm:h-8"
                    disabled={busy}
                    onClick={() => void showBindQR(a)}
                  >
                    <ScanLine className="size-4" />
                    扫码
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="h-10 sm:h-8"
                    disabled={busy}
                    onClick={() => void resetSecret(a)}
                  >
                    <RefreshCw className="size-4" />
                    重置
                  </Button>
                  <Button
                    size="sm"
                    variant="destructive"
                    className="h-10 sm:h-8"
                    disabled={busy}
                    onClick={() => void deleteAccount(a)}
                  >
                    <Trash2 className="size-4" />
                    删除
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))
        )}
      </div>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <form onSubmit={createAccount}>
            <DialogHeader>
              <DialogTitle>添加监控设备</DialogTitle>
              <DialogDescription>勾选本机可监听的支付渠道，同一设备可同时支持微信与支付宝。</DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>支持渠道</Label>
                <div className="grid grid-cols-2 gap-2">
                  {CHANNEL_OPTS.map((opt) => {
                    const on = picked.includes(opt.id)
                    return (
                      <button
                        key={opt.id}
                        type="button"
                        onClick={() => toggleChannel(opt.id)}
                        className={cn(
                          'rounded-lg border px-3 py-3 text-left text-sm font-semibold transition-colors',
                          on
                            ? 'border-primary bg-primary/10 text-foreground'
                            : 'border-border text-muted-foreground hover:bg-muted/60',
                        )}
                      >
                        <div>{opt.label}</div>
                        <div className="mt-1 text-xs font-normal opacity-80">{on ? '已启用' : '未启用'}</div>
                      </button>
                    )
                  })}
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="acc_name">设备备注</Label>
                <Input
                  id="acc_name"
                  className="h-11 sm:h-9"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="前台机 / 店员 A"
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>
                取消
              </Button>
              <Button type="submit" disabled={busy || picked.length === 0}>
                {busy ? '创建中…' : '创建并生成扫码'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={!!bind} onOpenChange={(open) => !open && setBind(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>App 扫码添加账号</DialogTitle>
            <DialogDescription className="sr-only">
              用 App 扫码导入。重置后旧 App 会失效。
            </DialogDescription>
          </DialogHeader>
          {bind ? (
            <div className="space-y-4">
              <div className="flex justify-center rounded-xl border bg-white p-3">
                {qrDataURL ? (
                  <img src={qrDataURL} alt="绑定二维码" className="aspect-square w-full max-w-[300px] sm:max-w-[320px]" />
                ) : (
                  <div className="flex aspect-square w-full max-w-[300px] items-center justify-center text-sm text-muted-foreground sm:max-w-[320px]">
                    生成中…
                  </div>
                )}
              </div>
              <div className="grid grid-cols-2 gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  className="h-10 sm:h-8"
                  onClick={() => copyText(JSON.stringify(bind), '配置已复制')}
                >
                  <Copy className="size-4" />
                  复制配置
                </Button>
                <Button size="sm" variant="outline" className="h-10 sm:h-8" onClick={() => copyText(bind.secret, '密钥已复制')}>
                  复制密钥
                </Button>
              </div>
            </div>
          ) : null}
          <DialogFooter>
            <Button onClick={() => setBind(null)}>完成</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Shell>
  )
}
