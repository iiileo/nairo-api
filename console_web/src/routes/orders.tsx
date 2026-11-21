import { Link, createFileRoute } from '@tanstack/react-router'
import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { Shell } from '@/components/Shell'
import { ApiError, api, type OrderItem } from '@/lib/api'
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

export const Route = createFileRoute('/orders')({
  beforeLoad: () => requireAuth(),
  component: OrdersPage,
})

function statusVariant(status: string): 'default' | 'secondary' | 'outline' | 'destructive' {
  if (status === 'paid') return 'default'
  if (status === 'pending') return 'secondary'
  if (status === 'expired') return 'outline'
  return 'destructive'
}

type StatusAction = 'paid' | 'closed' | 'expired'

function canEdit(s: string) {
  return s === 'pending' || s === 'expired'
}

function OrdersPage() {
  const [status, setStatus] = useState('all')
  const [channel, setChannel] = useState('all')
  const [settleType, setSettleType] = useState('all')
  const [keyword, setKeyword] = useState('')
  const [items, setItems] = useState<OrderItem[]>([])
  const [busy, setBusy] = useState(false)
  const [selected, setSelected] = useState<Set<string>>(() => new Set())
  const [target, setTarget] = useState<OrderItem | null>(null)
  const [batchIds, setBatchIds] = useState<string[] | null>(null)
  const [nextStatus, setNextStatus] = useState<StatusAction>('paid')
  const [remark, setRemark] = useState('')

  const editableItems = useMemo(() => items.filter((o) => canEdit(o.status)), [items])
  const selectedEditable = useMemo(
    () => editableItems.filter((o) => selected.has(o.id)),
    [editableItems, selected],
  )
  const allEditableSelected =
    editableItems.length > 0 && editableItems.every((o) => selected.has(o.id))
  const someEditableSelected = selectedEditable.length > 0 && !allEditableSelected

  async function load(overrides?: {
    status?: string
    channel?: string
    settle_type?: string
    q?: string
  }) {
    const next = {
      status: overrides?.status ?? status,
      channel: overrides?.channel ?? channel,
      settle_type: overrides?.settle_type ?? settleType,
      q: overrides?.q ?? keyword,
    }
    try {
      const res = await api.orders(next)
      setItems(res.items)
      setSelected(new Set())
    } catch {
      toast.error('加载订单失败')
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

  function toggleAllEditable(on: boolean) {
    setSelected((prev) => {
      const next = new Set(prev)
      for (const o of editableItems) {
        if (on) next.add(o.id)
        else next.delete(o.id)
      }
      return next
    })
  }

  function openStatus(o: OrderItem, st: StatusAction) {
    setBatchIds(null)
    setTarget(o)
    setNextStatus(st)
    setRemark(defaultRemark(st))
  }

  function openBatch(st: StatusAction) {
    const ids = selectedEditable
      .filter((o) => (st === 'expired' ? o.status === 'pending' : true))
      .map((o) => o.id)
    if (!ids.length) {
      toast.error(st === 'expired' ? '请选择待支付订单再标记过期' : '请先选择可处理订单')
      return
    }
    setTarget(null)
    setBatchIds(ids)
    setNextStatus(st)
    setRemark(defaultRemark(st, true))
  }

  function defaultRemark(st: StatusAction, batch = false) {
    const prefix = batch ? '批量：' : ''
    if (st === 'paid') return `${prefix}用户已付款但 App 未上报，人工确认`
    if (st === 'closed') return `${prefix}无效订单，手动关闭`
    return `${prefix}订单超时，标记过期`
  }

  async function submitStatus(e: FormEvent) {
    e.preventDefault()
    const ids = batchIds ?? (target ? [target.id] : [])
    if (!ids.length || !remark.trim()) return
    setBusy(true)
    try {
      const note = remark.trim()
      if (ids.length === 1 && !batchIds) {
        await api.setOrderStatus(ids[0], { status: nextStatus, remark: note })
        toast.success(nextStatus === 'paid' ? '已标记为已支付' : '状态已更新')
      } else {
        const res = await api.batchSetOrderStatus({
          ids,
          status: nextStatus,
          remark: note,
        })
        if (res.fail === 0) {
          toast.success(`已处理 ${res.ok} 笔`)
        } else {
          toast.error(`成功 ${res.ok} 笔，失败 ${res.fail} 笔`)
        }
      }
      setTarget(null)
      setBatchIds(null)
      setRemark('')
      setSelected(new Set())
      await load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.code : '更新失败')
    } finally {
      setBusy(false)
    }
  }

  const dialogOpen = !!target || !!batchIds
  const dialogCount = batchIds?.length ?? 1

  return (
    <Shell>
      <div className="mb-5 space-y-1 sm:mb-7">
        <h1 className="font-display text-2xl font-bold tracking-tight sm:text-3xl">订单</h1>
        <p className="text-sm text-muted-foreground">
          用户已付款但 App 未通知时，可手动改为「已支付」；也可关闭或标记过期。详见{' '}
          <Link to="/orphans" className="text-primary underline-offset-4 hover:underline">
            未匹配到账
          </Link>
          。
        </p>
      </div>

      <div className="mb-4 grid grid-cols-1 items-center gap-2 sm:grid-cols-2 lg:grid-cols-[minmax(0,1.4fr)_repeat(3,minmax(0,1fr))_auto]">
        <Input
          className="h-11 sm:h-9"
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') void load({ q: keyword })
          }}
          placeholder="商户单号"
        />
        <Select
          value={status}
          onValueChange={(v) => {
            const next = v || 'all'
            setStatus(next)
            void load({ status: next })
          }}
        >
          <SelectTrigger className="h-11 w-full sm:h-9">
            <SelectValue placeholder="状态" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部状态</SelectItem>
            <SelectItem value="pending">待支付</SelectItem>
            <SelectItem value="paid">已支付</SelectItem>
            <SelectItem value="expired">已过期</SelectItem>
            <SelectItem value="closed">已关闭</SelectItem>
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
        <Select
          value={settleType}
          onValueChange={(v) => {
            const next = v || 'all'
            setSettleType(next)
            void load({ settle_type: next })
          }}
        >
          <SelectTrigger className="h-11 w-full sm:h-9">
            <SelectValue placeholder="入账方式" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部入账</SelectItem>
            <SelectItem value="auto">自动入账</SelectItem>
            <SelectItem value="manual">人工补单</SelectItem>
          </SelectContent>
        </Select>
        <div className="flex gap-2 sm:col-span-2 lg:col-span-1">
          <Button className="h-11 flex-1 sm:h-9 lg:flex-none lg:px-5" onClick={() => void load()}>
            查询
          </Button>
          <Button
            className="h-11 sm:h-9"
            variant="outline"
            onClick={() => {
              setStatus('all')
              setChannel('all')
              setSettleType('all')
              setKeyword('')
              void load({ status: 'all', channel: 'all', settle_type: 'all', q: '' })
            }}
          >
            重置
          </Button>
        </div>
      </div>

      {selectedEditable.length > 0 ? (
        <div className="mb-4 flex flex-col gap-2 rounded-xl border bg-muted/40 p-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="text-sm font-medium">已选 {selectedEditable.length} 笔可处理订单</div>
          <div className="flex flex-wrap gap-2">
            <Button size="sm" disabled={busy} onClick={() => openBatch('paid')}>
              批量确认已付
            </Button>
            <Button size="sm" variant="outline" disabled={busy} onClick={() => openBatch('closed')}>
              批量关闭
            </Button>
            <Button size="sm" variant="outline" disabled={busy} onClick={() => openBatch('expired')}>
              批量过期
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
            <CardContent className="py-10 text-center text-muted-foreground">暂无订单</CardContent>
          </Card>
        ) : (
          <>
            {editableItems.length > 0 ? (
              <div className="flex items-center gap-2 px-1 text-sm text-muted-foreground">
                <Checkbox
                  checked={allEditableSelected ? true : someEditableSelected ? 'indeterminate' : false}
                  onCheckedChange={(v) => toggleAllEditable(v === true)}
                  aria-label="全选可处理订单"
                />
                <span>全选可处理（{editableItems.length}）</span>
              </div>
            ) : null}
            {items.map((o) => {
              const editable = canEdit(o.status)
              return (
                <Card key={o.id}>
                  <CardHeader className="space-y-2 pb-3">
                    <div className="flex items-start gap-3">
                      {editable ? (
                        <Checkbox
                          className="mt-1"
                          checked={selected.has(o.id)}
                          onCheckedChange={(v) => toggleOne(o.id, v === true)}
                          aria-label={`选择订单 ${o.out_trade_no}`}
                        />
                      ) : (
                        <span className="mt-1 size-4 shrink-0" />
                      )}
                      <div className="min-w-0 flex-1 space-y-2">
                        <div className="flex items-start justify-between gap-2">
                          <CardTitle className="break-all font-mono text-base">
                            {o.out_trade_no}
                          </CardTitle>
                          <Badge variant={statusVariant(o.status)}>
                            {labels.orderStatus(o.status)}
                          </Badge>
                        </div>
                        <p className="break-all font-mono text-xs text-muted-foreground">{o.id}</p>
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
                        <div className="text-muted-foreground">入账</div>
                        <div className="font-medium">{labels.settleType(o.settle_type)}</div>
                      </div>
                      <div>
                        <div className="text-muted-foreground">标价</div>
                        <div className="font-medium tabular-nums">
                          ¥ {yuanFromCents(o.amount_cents)}
                        </div>
                      </div>
                      <div>
                        <div className="text-muted-foreground">实付</div>
                        <div className="font-medium tabular-nums">
                          ¥ {yuanFromCents(o.pay_amount_cents)}
                        </div>
                      </div>
                    </div>
                    <div className="text-xs text-muted-foreground">{shortTime(o.paid_at)}</div>
                    {editable ? (
                      <div className="flex flex-wrap gap-2">
                        <Button className="flex-1" onClick={() => openStatus(o, 'paid')}>
                          确认已付
                        </Button>
                        <Button variant="outline" onClick={() => openStatus(o, 'closed')}>
                          关闭
                        </Button>
                        {o.status === 'pending' ? (
                          <Button variant="outline" onClick={() => openStatus(o, 'expired')}>
                            过期
                          </Button>
                        ) : null}
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
                    editableItems.length === 0
                      ? false
                      : allEditableSelected
                        ? true
                        : someEditableSelected
                          ? 'indeterminate'
                          : false
                  }
                  disabled={editableItems.length === 0}
                  onCheckedChange={(v) => toggleAllEditable(v === true)}
                  aria-label="全选可处理订单"
                />
              </TableHead>
              <TableHead>商户单号</TableHead>
              <TableHead>渠道</TableHead>
              <TableHead>标价</TableHead>
              <TableHead>实付</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>入账</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.length === 0 ? (
              <TableRow>
                <TableCell colSpan={8} className="h-24 text-center text-muted-foreground">
                  暂无订单
                </TableCell>
              </TableRow>
            ) : (
              items.map((o) => {
                const editable = canEdit(o.status)
                return (
                  <TableRow key={o.id} data-state={selected.has(o.id) ? 'selected' : undefined}>
                    <TableCell>
                      {editable ? (
                        <Checkbox
                          checked={selected.has(o.id)}
                          onCheckedChange={(v) => toggleOne(o.id, v === true)}
                          aria-label={`选择订单 ${o.out_trade_no}`}
                        />
                      ) : null}
                    </TableCell>
                    <TableCell>
                      <div className="font-mono text-sm">{o.out_trade_no}</div>
                      <div className="font-mono text-xs text-muted-foreground">{o.id}</div>
                    </TableCell>
                    <TableCell>{labels.channel(o.channel)}</TableCell>
                    <TableCell className="tabular-nums">¥ {yuanFromCents(o.amount_cents)}</TableCell>
                    <TableCell className="tabular-nums">
                      ¥ {yuanFromCents(o.pay_amount_cents)}
                    </TableCell>
                    <TableCell>
                      <Badge variant={statusVariant(o.status)}>
                        {labels.orderStatus(o.status)}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <span className="text-muted-foreground">
                        {labels.settleType(o.settle_type)}
                      </span>
                      <div className="mt-1 text-xs text-muted-foreground">
                        {shortTime(o.paid_at)}
                      </div>
                    </TableCell>
                    <TableCell>
                      {editable ? (
                        <div className="flex flex-wrap gap-1">
                          <Button size="sm" onClick={() => openStatus(o, 'paid')}>
                            确认已付
                          </Button>
                          <Button size="sm" variant="outline" onClick={() => openStatus(o, 'closed')}>
                            关闭
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

      <Dialog
        open={dialogOpen}
        onOpenChange={(v) => {
          if (!v) {
            setTarget(null)
            setBatchIds(null)
          }
        }}
      >
        <DialogContent className="max-h-[92dvh] overflow-y-auto sm:max-h-[90svh] sm:max-w-md">
          <form onSubmit={submitStatus}>
            <DialogHeader>
              <DialogTitle>
                {batchIds
                  ? nextStatus === 'paid'
                    ? '批量确认已付'
                    : nextStatus === 'closed'
                      ? '批量关闭'
                      : '批量标记过期'
                  : nextStatus === 'paid'
                    ? '手动确认已付'
                    : nextStatus === 'closed'
                      ? '关闭订单'
                      : '标记过期'}
              </DialogTitle>
              <DialogDescription>
                {batchIds ? (
                  <>将处理 {dialogCount} 笔订单。</>
                ) : (
                  <>
                    订单 <span className="font-mono">{target?.out_trade_no}</span> · ¥{' '}
                    {yuanFromCents(target?.pay_amount_cents)}
                    {nextStatus === 'paid'
                      ? '。适用于用户已付款但监控 App 未推送通知的情况。'
                      : null}
                  </>
                )}
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-3 py-4">
              {!batchIds ? (
                <div className="space-y-2">
                  <Label>目标状态</Label>
                  <Select
                    value={nextStatus}
                    onValueChange={(v) => setNextStatus((v as StatusAction) || 'paid')}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="paid">已支付</SelectItem>
                      <SelectItem value="closed">已关闭</SelectItem>
                      <SelectItem value="expired">已过期</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              ) : null}
              <div className="space-y-2">
                <Label htmlFor="remark">备注（必填）</Label>
                <Textarea
                  id="remark"
                  rows={3}
                  value={remark}
                  onChange={(e) => setRemark(e.target.value)}
                  required
                  placeholder="说明核对依据，如微信/支付宝流水号"
                />
              </div>
            </div>
            <DialogFooter className="gap-2 sm:gap-0">
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setTarget(null)
                  setBatchIds(null)
                }}
                disabled={busy}
              >
                取消
              </Button>
              <Button type="submit" disabled={busy || !remark.trim()}>
                {busy ? '提交中…' : batchIds ? `确认处理 ${dialogCount} 笔` : '确认修改'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </Shell>
  )
}
