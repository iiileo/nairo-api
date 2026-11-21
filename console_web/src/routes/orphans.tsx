import { Link, createFileRoute } from '@tanstack/react-router'
import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Shell } from '@/components/Shell'
import { ApiError, api, type OrphanItem } from '@/lib/api'
import { shortTime, yuanFromCents } from '@/lib/format'
import { labels } from '@/lib/labels'
import { requireAuth } from '@/lib/requireAuth'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'

export const Route = createFileRoute('/orphans')({
  beforeLoad: () => requireAuth(),
  component: OrphansPage,
})

function OrphansPage() {
  const [status, setStatus] = useState('open')
  const [channel, setChannel] = useState('all')
  const [items, setItems] = useState<OrphanItem[]>([])
  const [busy, setBusy] = useState(false)
  const [selected, setSelected] = useState<Set<string>>(() => new Set())
  const [target, setTarget] = useState<OrphanItem | null>(null)
  const [batchIgnoreIds, setBatchIgnoreIds] = useState<string[] | null>(null)
  const [orderId, setOrderId] = useState('')
  const [remark, setRemark] = useState('')

  const openItems = useMemo(() => items.filter((o) => o.status === 'open'), [items])
  const selectedOpen = useMemo(
    () => openItems.filter((o) => selected.has(o.id)),
    [openItems, selected],
  )
  const allOpenSelected = openItems.length > 0 && openItems.every((o) => selected.has(o.id))
  const someOpenSelected = selectedOpen.length > 0 && !allOpenSelected

  async function load(overrides?: { status?: string; channel?: string }) {
    const next = {
      status: overrides?.status ?? status,
      channel: overrides?.channel ?? channel,
    }
    try {
      const res = await api.orphans(next)
      setItems(res.items)
      setSelected(new Set())
    } catch {
      toast.error('加载未匹配到账失败')
    }
  }

  useEffect(() => {
    void load()
  }, [])

  function toggleOne(id: string, on: boolean) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (on) next.add(id)
      else next.delete(id)
      return next
    })
  }

  function toggleAllOpen(on: boolean) {
    setSelected((prev) => {
      const next = new Set(prev)
      for (const o of openItems) {
        if (on) next.add(o.id)
        else next.delete(o.id)
      }
      return next
    })
  }

  async function submitBind(e: FormEvent) {
    e.preventDefault()
    if (!target) return
    setBusy(true)
    try {
      await api.bindOrphan(target.id, {
        order_id: orderId.trim(),
        remark: remark.trim(),
      })
      toast.success('绑定成功')
      setTarget(null)
      setOrderId('')
      setRemark('')
      await load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.code : '绑定失败')
    } finally {
      setBusy(false)
    }
  }

  function ignoreOne(o: OrphanItem) {
    setRemark('手动忽略')
    setBatchIgnoreIds([o.id])
  }

  function openBatchIgnore() {
    const ids = selectedOpen.map((o) => o.id)
    if (!ids.length) {
      toast.error('请先选择待处理记录')
      return
    }
    setRemark(`批量忽略 ${ids.length} 笔未匹配到账`)
    setBatchIgnoreIds(ids)
  }

  async function submitIgnore(e: FormEvent) {
    e.preventDefault()
    if (!batchIgnoreIds?.length || !remark.trim()) return
    setBusy(true)
    try {
      const note = remark.trim()
      if (batchIgnoreIds.length === 1) {
        await api.ignoreOrphan(batchIgnoreIds[0], note)
        toast.success('已忽略')
      } else {
        const res = await api.batchIgnoreOrphans({ ids: batchIgnoreIds, remark: note })
        if (res.fail === 0) {
          toast.success(`已忽略 ${res.ok} 笔`)
        } else {
          toast.error(`成功 ${res.ok} 笔，失败 ${res.fail} 笔`)
        }
      }
      setBatchIgnoreIds(null)
      setRemark('')
      setSelected(new Set())
      await load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.code : '操作失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Shell>
      <div className="mb-5 space-y-1 sm:mb-7">
        <h1 className="font-display text-2xl font-bold tracking-tight sm:text-3xl">未匹配到账</h1>
        <p className="text-sm text-muted-foreground">
          App 上报但未命中订单的金额，可绑定订单补单。若是用户已付款但 App
          完全没通知，请到{' '}
          <Link to="/orders" className="text-primary underline-offset-4 hover:underline">
            订单
          </Link>{' '}
          手动改状态为已支付。
        </p>
      </div>

      <div className="mb-4 grid grid-cols-1 items-center gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
        <Select
          value={status}
          onValueChange={(v) => {
            const next = v || 'open'
            setStatus(next)
            void load({ status: next })
          }}
        >
          <SelectTrigger className="h-11 w-full sm:h-9">
            <SelectValue placeholder="状态" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="open">待处理</SelectItem>
            <SelectItem value="bound">已绑定</SelectItem>
            <SelectItem value="ignored">已忽略</SelectItem>
            <SelectItem value="all">全部状态</SelectItem>
          </SelectContent>
        </Select>
        <Select
          value={channel}
          onValueChange={(v) => {
            const next = v || 'all'
            setChannel(next)
            void load({ channel: next })
          }}
        >
          <SelectTrigger className="h-11 w-full sm:h-9">
            <SelectValue placeholder="渠道" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部渠道</SelectItem>
            <SelectItem value="wechat">微信</SelectItem>
            <SelectItem value="alipay">支付宝</SelectItem>
          </SelectContent>
        </Select>
        <div className="flex gap-2">
          <Button
            className="h-11 flex-1 sm:h-9 sm:flex-none sm:px-5"
            variant="outline"
            onClick={() => void load()}
          >
            刷新
          </Button>
          <Button
            className="h-11 sm:h-9"
            variant="outline"
            onClick={() => {
              setStatus('open')
              setChannel('all')
              void load({ status: 'open', channel: 'all' })
            }}
          >
            重置
          </Button>
        </div>
      </div>

      {selectedOpen.length > 0 ? (
        <div className="mb-4 flex flex-col gap-2 rounded-xl border bg-muted/40 p-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="text-sm font-medium">已选 {selectedOpen.length} 笔待处理</div>
          <div className="flex flex-wrap gap-2">
            <Button size="sm" variant="outline" disabled={busy} onClick={openBatchIgnore}>
              批量忽略
            </Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={busy}
              onClick={() => setSelected(new Set())}
            >
              清空
            </Button>
          </div>
        </div>
      ) : null}

      <div className="space-y-3 md:hidden">
        {items.length === 0 ? (
          <Card>
            <CardContent className="py-10 text-center text-muted-foreground">
              暂无未匹配记录
            </CardContent>
          </Card>
        ) : (
          <>
            {openItems.length > 0 ? (
              <div className="flex items-center gap-2 px-1 text-sm text-muted-foreground">
                <Checkbox
                  checked={allOpenSelected ? true : someOpenSelected ? 'indeterminate' : false}
                  onCheckedChange={(v) => toggleAllOpen(v === true)}
                  aria-label="全选待处理"
                />
                <span>全选待处理（{openItems.length}）</span>
              </div>
            ) : null}
            {items.map((o) => {
              const isOpen = o.status === 'open'
              return (
                <Card key={o.id}>
                  <CardHeader className="space-y-2 pb-3">
                    <div className="flex items-start gap-3">
                      {isOpen ? (
                        <Checkbox
                          className="mt-1"
                          checked={selected.has(o.id)}
                          onCheckedChange={(v) => toggleOne(o.id, v === true)}
                          aria-label={`选择未匹配 ${o.id}`}
                        />
                      ) : (
                        <span className="mt-1 size-4 shrink-0" />
                      )}
                      <div className="min-w-0 flex-1 space-y-2">
                        <div className="flex items-start justify-between gap-2">
                          <CardTitle className="break-all font-mono text-sm">{o.id}</CardTitle>
                          <Badge variant="secondary">{labels.orphanStatus(o.status)}</Badge>
                        </div>
                      </div>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-3 text-sm">
                    <div className="grid grid-cols-2 gap-2">
                      <div>
                        <div className="text-muted-foreground">渠道</div>
                        <div className="font-medium">{labels.channel(o.channel)}</div>
                      </div>
                      <div>
                        <div className="text-muted-foreground">金额</div>
                        <div className="font-medium tabular-nums">
                          ¥ {yuanFromCents(o.pay_amount_cents)}
                        </div>
                      </div>
                    </div>
                    <div className="text-xs text-muted-foreground">{shortTime(o.reported_at)}</div>
                    {isOpen ? (
                      <div className="flex gap-2">
                        <Button
                          className="flex-1"
                          onClick={() => {
                            setTarget(o)
                            setOrderId('')
                            setRemark('')
                          }}
                        >
                          绑定订单
                        </Button>
                        <Button
                          variant="outline"
                          disabled={busy}
                          onClick={() => void ignoreOne(o)}
                        >
                          忽略
                        </Button>
                      </div>
                    ) : null}
                  </CardContent>
                </Card>
              )
            })}
          </>
        )}
      </div>

      <div className="hidden overflow-hidden rounded-xl border bg-card md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox
                  checked={
                    openItems.length === 0
                      ? false
                      : allOpenSelected
                        ? true
                        : someOpenSelected
                          ? 'indeterminate'
                          : false
                  }
                  disabled={openItems.length === 0}
                  onCheckedChange={(v) => toggleAllOpen(v === true)}
                  aria-label="全选待处理"
                />
              </TableHead>
              <TableHead>ID</TableHead>
              <TableHead>渠道</TableHead>
              <TableHead>金额</TableHead>
              <TableHead>上报时间</TableHead>
              <TableHead>状态</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className="h-24 text-center text-muted-foreground">
                  暂无未匹配记录
                </TableCell>
              </TableRow>
            ) : (
              items.map((o) => {
                const isOpen = o.status === 'open'
                return (
                  <TableRow key={o.id} data-state={selected.has(o.id) ? 'selected' : undefined}>
                    <TableCell>
                      {isOpen ? (
                        <Checkbox
                          checked={selected.has(o.id)}
                          onCheckedChange={(v) => toggleOne(o.id, v === true)}
                          aria-label={`选择未匹配 ${o.id}`}
                        />
                      ) : null}
                    </TableCell>
                    <TableCell className="font-mono text-xs">{o.id}</TableCell>
                    <TableCell>{labels.channel(o.channel)}</TableCell>
                    <TableCell className="tabular-nums">
                      ¥ {yuanFromCents(o.pay_amount_cents)}
                    </TableCell>
                    <TableCell>{shortTime(o.reported_at)}</TableCell>
                    <TableCell>
                      <Badge variant="secondary">{labels.orphanStatus(o.status)}</Badge>
                    </TableCell>
                    <TableCell>
                      {isOpen ? (
                        <div className="flex flex-wrap gap-1">
                          <Button
                            size="sm"
                            onClick={() => {
                              setTarget(o)
                              setOrderId('')
                              setRemark('')
                            }}
                          >
                            绑定订单
                          </Button>
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={busy}
                            onClick={() => void ignoreOne(o)}
                          >
                            忽略
                          </Button>
                        </div>
                      ) : null}
                    </TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog open={!!target} onOpenChange={(v) => !v && setTarget(null)}>
        <DialogContent className="max-h-[92dvh] overflow-y-auto sm:max-h-[90svh] sm:max-w-md">
          <form onSubmit={submitBind}>
            <DialogHeader>
              <DialogTitle>绑定到订单</DialogTitle>
              <DialogDescription>
                孤儿单金额 ¥ {yuanFromCents(target?.pay_amount_cents)} ·{' '}
                {labels.channel(target?.channel)}
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="order_id">订单号</Label>
                <Input
                  id="order_id"
                  className="font-mono"
                  value={orderId}
                  onChange={(e) => setOrderId(e.target.value)}
                  required
                  placeholder="从订单列表复制 id"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="bind_remark">备注（必填）</Label>
                <Textarea
                  id="bind_remark"
                  rows={3}
                  value={remark}
                  onChange={(e) => setRemark(e.target.value)}
                  required
                />
              </div>
            </div>
            <DialogFooter className="gap-2 sm:gap-0">
              <Button type="button" variant="outline" disabled={busy} onClick={() => setTarget(null)}>
                取消
              </Button>
              <Button type="submit" disabled={busy || !orderId.trim() || !remark.trim()}>
                {busy ? '提交中…' : '确认绑定'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog
        open={!!batchIgnoreIds}
        onOpenChange={(v) => {
          if (!v) setBatchIgnoreIds(null)
        }}
      >
        <DialogContent className="max-h-[92dvh] overflow-y-auto sm:max-h-[90svh] sm:max-w-md">
          <form onSubmit={submitIgnore}>
            <DialogHeader>
              <DialogTitle>
                {(batchIgnoreIds?.length ?? 0) > 1 ? '批量忽略' : '忽略未匹配到账'}
              </DialogTitle>
              <DialogDescription>
                将忽略 {batchIgnoreIds?.length ?? 0} 笔记录，不会修改任何订单状态。
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="ignore_remark">备注（必填）</Label>
                <Textarea
                  id="ignore_remark"
                  rows={3}
                  value={remark}
                  onChange={(e) => setRemark(e.target.value)}
                  required
                />
              </div>
            </div>
            <DialogFooter className="gap-2 sm:gap-0">
              <Button
                type="button"
                variant="outline"
                disabled={busy}
                onClick={() => setBatchIgnoreIds(null)}
              >
                取消
              </Button>
              <Button type="submit" disabled={busy || !remark.trim()}>
                {busy
                  ? '提交中…'
                  : (batchIgnoreIds?.length ?? 0) > 1
                    ? `确认忽略 ${batchIgnoreIds?.length} 笔`
                    : '确认忽略'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </Shell>
  )
}
