import { Link, createFileRoute, useNavigate } from '@tanstack/react-router'
import { ArrowLeft, ImagePlus, Plus, Trash2, X } from 'lucide-react'
import QRCode from 'qrcode'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { ChannelBrandIcon } from '@/components/ChannelIcons'
import { ChannelPicker } from '@/components/ChannelPicker'
import { PayCodeThumb } from '@/components/PayCodeThumb'
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
import { ApiError, api, type AccountItem, type PayCodeItem } from '@/lib/api'
import { accountChannels, channelBrand } from '@/lib/channels'
import { decodeQrFromFile } from '@/lib/decodeQr'
import { yuanFromCents } from '@/lib/format'
import { labels } from '@/lib/labels'
import { requireAuth } from '@/lib/requireAuth'
import { cn } from '@/lib/utils'
import { useConfirm } from '@/components/ConfirmDialog'

type CodeTab = 'fixed' | 'float'

/** 待提交的收款码草稿（一张二维码图片对应一条）。 */
type DraftCode = {
  key: string
  previewURL: string
  payload: string
  amountYuan: string
  error?: string
}

export const Route = createFileRoute('/accounts_/$accountId')({
  beforeLoad: () => requireAuth(),
  component: AccountPayCodesPage,
})

function AccountPayCodesPage() {
  const confirm = useConfirm()
  const { accountId } = Route.useParams()
  const navigate = useNavigate()
  const [account, setAccount] = useState<AccountItem | null>(null)
  const [codes, setCodes] = useState<PayCodeItem[]>([])
  const [loading, setLoading] = useState(true)
  const [codeOpen, setCodeOpen] = useState(false)
  const [codeType, setCodeType] = useState('float')
  const [codeChannel, setCodeChannel] = useState('wechat')
  const [listTab, setListTab] = useState<CodeTab>('float')
  const [drafts, setDrafts] = useState<DraftCode[]>([])
  const [decoding, setDecoding] = useState(false)
  const [bulkAmount, setBulkAmount] = useState('')
  const [busy, setBusy] = useState(false)
  const [previewCode, setPreviewCode] = useState<PayCodeItem | null>(null)
  const [deletingId, setDeletingId] = useState('')
  const [previewSrc, setPreviewSrc] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  const fixedCodes = useMemo(
    () =>
      codes
        .filter((c) => c.code_type === 'fixed')
        .slice()
        .sort((a, b) => (a.amount_cents ?? 0) - (b.amount_cents ?? 0)),
    [codes],
  )
  const floatCodes = useMemo(() => codes.filter((c) => c.code_type !== 'fixed'), [codes])
  const tabCodes = listTab === 'fixed' ? fixedCodes : floatCodes

  function openAddDialog(preferredType?: CodeTab) {
    if (!account) return
    const enabled = accountChannels(account)
    setCodeChannel(enabled[0] || 'wechat')
    setCodeType(preferredType || listTab)
    setCodeOpen(true)
  }

  async function load() {
    setLoading(true)
    try {
      const [accRes, codeRes] = await Promise.all([api.accounts(), api.payCodes(accountId)])
      const found = accRes.items.find((a) => a.id === accountId) || null
      if (!found) {
        toast.error('设备不存在')
        void navigate({ to: '/accounts' })
        return
      }
      setAccount(found)
      setCodes(codeRes.items)
      const enabled = accountChannels(found)
      if (enabled.length) setCodeChannel(enabled[0])
    } catch {
      toast.error('加载收款码失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [accountId])

  useEffect(() => {
    let cancelled = false
    const text = (previewCode?.payload || previewCode?.image_url || '').trim()
    if (!text) {
      setPreviewSrc('')
      return
    }
    if (/\.(png|jpe?g|webp|gif)(\?|$)/i.test(text) || text.startsWith('data:image')) {
      setPreviewSrc(text)
      return
    }
    void QRCode.toDataURL(text, { width: 280, margin: 1 }).then(
      (url) => {
        if (!cancelled) setPreviewSrc(url)
      },
      () => {
        if (!cancelled) setPreviewSrc('')
      },
    )
    return () => {
      cancelled = true
    }
  }, [previewCode])

  function resetDialog() {
    setDrafts((prev) => {
      prev.forEach((d) => URL.revokeObjectURL(d.previewURL))
      return []
    })
    setBulkAmount('')
  }

  /** 批量识别图片（文件选择 / 粘贴 / 拖拽共用）。 */
  async function addFiles(files: File[]) {
    const images = files.filter((f) => f.type.startsWith('image/'))
    if (!images.length) return
    setDecoding(true)
    const decoded: DraftCode[] = []
    let failed = 0
    for (const file of images) {
      try {
        const text = await decodeQrFromFile(file)
        decoded.push({
          key: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
          previewURL: URL.createObjectURL(file),
          payload: text,
          amountYuan: '',
        })
      } catch {
        failed++
      }
    }
    let skipped = 0
    let added = 0
    setDrafts((prev) => {
      const known = new Set(prev.map((d) => d.payload))
      const next = [...prev]
      for (const d of decoded) {
        if (known.has(d.payload) || (codeType !== 'fixed' && next.length >= 1)) {
          URL.revokeObjectURL(d.previewURL)
          skipped++
          continue
        }
        known.add(d.payload)
        next.push(d)
        added++
      }
      return next
    })
    setDecoding(false)
    if (fileRef.current) fileRef.current.value = ''
    if (failed) toast.error(`${failed} 张图片未识别到二维码`)
    if (skipped) {
      toast.info(
        codeType === 'fixed'
          ? `已跳过 ${skipped} 张重复二维码`
          : '任意金额码每个渠道只能 1 张，已忽略多余图片',
      )
    }
    if (decoded.length - skipped > 0) toast.success(`已识别 ${decoded.length - skipped} 张二维码`)
  }

  // 弹窗打开时监听 Ctrl/⌘+V：支持一次粘贴多张图片
  useEffect(() => {
    if (!codeOpen) return
    function onPaste(e: ClipboardEvent) {
      const data = e.clipboardData
      if (!data) return
      const files = Array.from(data.files).filter((f) => f.type.startsWith('image/'))
      if (!files.length) {
        for (const item of Array.from(data.items)) {
          if (item.kind === 'file' && item.type.startsWith('image/')) {
            const f = item.getAsFile()
            if (f) files.push(f)
          }
        }
      }
      if (!files.length) return
      e.preventDefault()
      void addFiles(files)
    }
    document.addEventListener('paste', onPaste)
    return () => document.removeEventListener('paste', onPaste)
  }, [codeOpen, codeType])

  function changeCodeType(next: 'fixed' | 'float') {
    setCodeType(next)
    if (next !== 'fixed') {
      setDrafts((prev) => {
        prev.slice(1).forEach((d) => URL.revokeObjectURL(d.previewURL))
        return prev.slice(0, 1).map((d) => ({ ...d, error: undefined }))
      })
    }
  }

  function updateDraft(key: string, patch: Partial<DraftCode>) {
    setDrafts((prev) => prev.map((d) => (d.key === key ? { ...d, ...patch, error: undefined } : d)))
  }

  function removeDraft(key: string) {
    setDrafts((prev) => {
      const hit = prev.find((d) => d.key === key)
      if (hit) URL.revokeObjectURL(hit.previewURL)
      return prev.filter((d) => d.key !== key)
    })
  }

  /** 批量设置金额：填 1 个金额 → 应用到全部；填多个（空格/逗号/换行分隔）→ 按顺序依次分配。 */
  function applyBulkAmount() {
    const tokens = bulkAmount
      .split(/[\s,，;；、]+/)
      .map((t) => t.trim())
      .filter(Boolean)
    if (!tokens.length || tokens.some((t) => !(Number(t) > 0))) {
      toast.error('请输入有效金额，如 5 或 1 2 5 10')
      return
    }
    if (tokens.length > 1 && tokens.length !== drafts.length) {
      toast.info(`金额 ${tokens.length} 个、二维码 ${drafts.length} 张，将按顺序分配`)
    }
    setDrafts((prev) =>
      prev.map((d, i) => {
        const t = tokens.length === 1 ? tokens[0] : tokens[i]
        return t ? { ...d, amountYuan: t, error: undefined } : d
      }),
    )
  }

  async function deleteCode(c: PayCodeItem) {
    const desc = `${labels.channel(c.channel)} ${
      c.code_type === 'fixed' ? `固额 ¥ ${yuanFromCents(c.amount_cents)}` : '任意金额'
    }`
    const ok = await confirm({
      title: '删除收款码',
      description: `确认删除收款码「${desc}」？删除后不再参与分配，已生成的订单不受影响。`,
      confirmText: '删除',
      destructive: true,
    })
    if (!ok) return
    setDeletingId(c.id)
    try {
      await api.deletePayCode(c.id)
      toast.success('收款码已删除')
      setCodes((prev) => prev.filter((x) => x.id !== c.id))
      setPreviewCode((cur) => (cur?.id === c.id ? null : cur))
    } catch (err) {
      toast.error(err instanceof ApiError ? err.detail || err.code : '删除失败')
    } finally {
      setDeletingId('')
    }
  }

  async function createCodes(e: FormEvent) {
    e.preventDefault()
    if (!account) return
    if (!codeChannel) {
      toast.error('请选择收款码渠道')
      return
    }
    if (!drafts.length) {
      toast.error('请先选择、粘贴或拖入收款码图片')
      return
    }
    const fixed = codeType === 'fixed'
    const seen = new Set<number>()
    let invalid = false
    const checked = drafts.map((d, i) => {
      let error: string | undefined
      let cents: number | undefined
      if (fixed) {
        const yuan = Number(d.amountYuan)
        if (!d.amountYuan.trim() || !(yuan > 0)) {
          error = '请填写金额'
        } else {
          cents = Math.round(yuan * 100)
          if (seen.has(cents)) error = '与本批次其他码金额重复'
          else if (
            codes.some(
              (c) => c.code_type === 'fixed' && c.channel === codeChannel && c.amount_cents === cents,
            )
          )
            error = '该渠道已有同金额固额码'
          seen.add(cents)
        }
      } else if (i > 0) {
        error = '任意金额码每个渠道只能 1 张'
      } else if (codes.some((c) => c.code_type !== 'fixed' && c.channel === codeChannel)) {
        error = '该渠道已有任意金额码，请先删除'
      }
      if (error) invalid = true
      return { ...d, error, cents }
    })
    setDrafts(checked.map(({ cents: _c, ...d }) => d))
    if (invalid) {
      toast.error('请先修正标红的二维码')
      return
    }

    setBusy(true)
    let ok = 0
    const remain: DraftCode[] = []
    for (const d of checked) {
      try {
        await api.createPayCode({
          account_id: account.id,
          channel: codeChannel,
          code_type: codeType,
          payload: d.payload.trim(),
          amount_cents: fixed ? d.cents : undefined,
        })
        ok++
        URL.revokeObjectURL(d.previewURL)
      } catch (err) {
        const { cents: _c, ...rest } = d
        remain.push({
          ...rest,
          error: err instanceof ApiError ? err.detail || err.code : '添加失败',
        })
      }
    }
    try {
      const res = await api.payCodes(account.id)
      setCodes(res.items)
    } catch {
      /* 列表刷新失败不影响结果提示 */
    }
    setBusy(false)
    if (ok > 0) setListTab(fixed ? 'fixed' : 'float')
    if (remain.length === 0) {
      toast.success(ok > 1 ? `已添加 ${ok} 张收款码` : '收款码已添加')
      setCodeOpen(false)
      resetDialog()
    } else {
      setDrafts(remain)
      toast.error(`成功 ${ok} 张，失败 ${remain.length} 张，请查看标红项`)
    }
  }

  const title =
    account?.name || account?.channel_label || (account ? labels.channel(account.channel) : '收款码')
  const previewBrand = previewCode ? channelBrand(previewCode.channel) : null

  return (
    <Shell>
      <div className="mb-5 space-y-3 sm:mb-7">
        <Button asChild variant="ghost" className="-ml-2 h-10 w-fit px-2 sm:h-9">
          <Link to="/accounts">
            <ArrowLeft className="size-4" />
            返回设备列表
          </Link>
        </Button>
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div className="space-y-1">
            <h1 className="font-display text-2xl font-bold tracking-tight sm:text-3xl">收款码</h1>
            <p className="text-sm text-muted-foreground">
              设备：{loading ? '加载中…' : title}
              {account ? ` · ${account.channel_label || labels.channels(account.channels)}` : ''}
            </p>
          </div>
          <Button
            className="h-11 w-full sm:h-9 sm:w-auto"
            disabled={!account}
            onClick={() => openAddDialog()}
          >
            <Plus className="size-4" />
            添加收款码
          </Button>
        </div>
      </div>

      {loading ? (
        <Card>
          <CardContent className="py-12 text-center text-sm text-muted-foreground">加载中…</CardContent>
        </Card>
      ) : codes.length === 0 ? (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">暂无收款码</CardTitle>
            <CardDescription>按微信 / 支付宝分别上传任意金额码或固额码。</CardDescription>
          </CardHeader>
          <CardContent>
            <Button className="h-11 w-full sm:h-9 sm:w-auto" onClick={() => openAddDialog('float')}>
              <Plus className="size-4" />
              添加第一张收款码
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          <div
            className="grid grid-cols-2 gap-1 rounded-xl border bg-muted/40 p-1"
            role="tablist"
            aria-label="收款码类型"
          >
            {(
              [
                { id: 'float' as const, label: '任意金额', count: floatCodes.length },
                { id: 'fixed' as const, label: '固额', count: fixedCodes.length },
              ] as const
            ).map((tab) => {
              const on = listTab === tab.id
              return (
                <button
                  key={tab.id}
                  type="button"
                  role="tab"
                  aria-selected={on}
                  onClick={() => setListTab(tab.id)}
                  className={cn(
                    'flex h-10 items-center justify-center gap-2 rounded-lg text-sm font-semibold transition-colors',
                    on
                      ? 'bg-background text-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground',
                  )}
                >
                  <span>{tab.label}</span>
                  <span
                    className={cn(
                      'rounded-full px-1.5 py-0.5 text-[11px] tabular-nums',
                      on ? 'bg-muted text-foreground' : 'bg-background/70 text-muted-foreground',
                    )}
                  >
                    {tab.count}
                  </span>
                </button>
              )
            })}
          </div>

          {tabCodes.length === 0 ? (
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">
                  {listTab === 'fixed' ? '暂无固额码' : '暂无任意金额码'}
                </CardTitle>
                <CardDescription>
                  {listTab === 'fixed'
                    ? '固额码优先匹配同金额订单；可按常用金额各上传一张。'
                    : '任意金额码作兜底；建议微信、支付宝各至少一张。'}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Button
                  className="h-11 w-full sm:h-9 sm:w-auto"
                  onClick={() => openAddDialog(listTab)}
                >
                  <Plus className="size-4" />
                  添加{listTab === 'fixed' ? '固额' : '任意金额'}码
                </Button>
              </CardContent>
            </Card>
          ) : (
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
              {tabCodes.map((c) => {
                const brand = channelBrand(c.channel)
                return (
                  <div
                    key={c.id}
                    className={cn(
                      'flex flex-col overflow-hidden rounded-xl border border-l-4 bg-card shadow-sm',
                      brand.card,
                    )}
                  >
                  <button
                    type="button"
                    className="flex flex-col gap-3 p-4 text-left transition-colors hover:bg-muted/30"
                    onClick={() => setPreviewCode(c)}
                  >
                    <div className="flex items-start justify-between gap-2">
                      <span
                        className={cn(
                          'inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-semibold',
                          brand.chip,
                        )}
                      >
                        <ChannelBrandIcon channel={c.channel} className={cn('size-3.5', brand.icon)} />
                        {labels.channel(c.channel) || '未标注渠道'}
                      </span>
                      <Badge variant="secondary">{labels.accountStatus(c.status)}</Badge>
                    </div>
                    <div className="flex justify-center">
                      <PayCodeThumb content={c.payload || c.image_url} large />
                    </div>
                    <div className="space-y-1 text-sm">
                      {c.code_type === 'fixed' ? (
                        <div className="font-display text-xl font-semibold tabular-nums tracking-tight">
                          ¥ {yuanFromCents(c.amount_cents)}
                        </div>
                      ) : (
                        <div className="font-medium text-muted-foreground">任意金额</div>
                      )}
                      <div className="line-clamp-2 break-all font-mono text-[11px] text-muted-foreground">
                        {c.payload || c.image_url || '—'}
                      </div>
                    </div>
                  </button>
                  <div className="flex justify-end border-t px-3 py-2">
                    <Button
                      type="button"
                      variant="ghost"
                      className="h-9 gap-1.5 px-2 text-destructive hover:text-destructive sm:h-8"
                      disabled={deletingId === c.id}
                      onClick={() => void deleteCode(c)}
                    >
                      <Trash2 className="size-3.5" />
                      {deletingId === c.id ? '删除中…' : '删除'}
                    </Button>
                  </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      )}

      <Dialog
        open={!!previewCode}
        onOpenChange={(open) => {
          if (!open) {
            setPreviewCode(null)
            setPreviewSrc('')
          }
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              {previewCode ? (
                <span
                  className={cn(
                    'inline-flex size-8 items-center justify-center rounded-lg border',
                    previewBrand?.chip,
                  )}
                >
                  <ChannelBrandIcon
                    channel={previewCode.channel}
                    className={cn('size-4', previewBrand?.icon)}
                  />
                </span>
              ) : null}
              收款码预览
            </DialogTitle>
            <DialogDescription>
              {previewCode
                ? `${labels.channel(previewCode.channel)} · ${labels.codeType(previewCode.code_type)}${
                    previewCode.code_type === 'fixed'
                      ? ` · ¥ ${yuanFromCents(previewCode.amount_cents)}`
                      : ''
                  }`
                : ''}
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-center rounded-xl border bg-white p-4">
            {previewSrc ? (
              <img src={previewSrc} alt="收款码" className="size-[280px] max-w-full object-contain" />
            ) : (
              <div className="flex size-[280px] max-w-full items-center justify-center text-sm text-muted-foreground">
                生成中…
              </div>
            )}
          </div>
          <div className="break-all font-mono text-xs text-muted-foreground">
            {previewCode?.payload || previewCode?.image_url || '—'}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              className="text-destructive hover:text-destructive"
              disabled={!previewCode || deletingId === previewCode.id}
              onClick={() => previewCode && void deleteCode(previewCode)}
            >
              <Trash2 className="size-4" />
              删除
            </Button>
            <Button type="button" variant="outline" onClick={() => setPreviewCode(null)}>
              关闭
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={codeOpen}
        onOpenChange={(open) => {
          setCodeOpen(open)
          if (!open) resetDialog()
        }}
      >
        <DialogContent className="flex max-h-[92dvh] flex-col gap-0 overflow-hidden p-0 sm:max-h-[88dvh] sm:max-w-2xl sm:p-0">
          <form onSubmit={createCodes} className="flex min-h-0 flex-1 flex-col">
            <DialogHeader className="border-b px-5 py-4 pr-12">
              <DialogTitle>添加收款码</DialogTitle>
              <DialogDescription>
                选择、拖入或 Ctrl/⌘ + V 粘贴二维码图片，浏览器本地识别。
              </DialogDescription>
            </DialogHeader>

            <div
              className="min-h-0 flex-1 space-y-4 overflow-y-auto px-5 py-4"
              onDragOver={(e) => e.preventDefault()}
              onDrop={(e) => {
                e.preventDefault()
                void addFiles(Array.from(e.dataTransfer.files))
              }}
            >
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="space-y-1.5">
                  <Label>收款渠道</Label>
                  <ChannelPicker
                    className="[&>button]:h-10"
                    value={codeChannel}
                    onChange={setCodeChannel}
                    enabledIds={account ? accountChannels(account) : undefined}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label>类型</Label>
                  <div className="grid grid-cols-2 gap-2" role="group" aria-label="收款码类型">
                    {(
                      [
                        { id: 'float', label: '任意金额', hint: '仅 1 张' },
                        { id: 'fixed', label: '固额', hint: '支持批量' },
                      ] as const
                    ).map((opt) => {
                      const on = codeType === opt.id
                      return (
                        <button
                          key={opt.id}
                          type="button"
                          aria-pressed={on}
                          onClick={() => changeCodeType(opt.id)}
                          className={cn(
                            'flex h-10 items-center justify-center gap-1.5 rounded-xl border px-3 text-sm font-semibold transition-colors',
                            on
                              ? 'border-primary bg-primary/10 text-primary'
                              : 'border-border bg-card text-muted-foreground hover:bg-muted/60 hover:text-foreground',
                          )}
                        >
                          {opt.label}
                          <span className="text-[11px] font-normal opacity-70">{opt.hint}</span>
                        </button>
                      )
                    })}
                  </div>
                </div>
              </div>
              <p className="-mt-1 text-xs text-muted-foreground">
                同渠道仅可有 1 张任意金额码；同金额固额码也仅 1 张，更换请先删除旧码。
              </p>

              <input
                ref={fileRef}
                type="file"
                accept="image/*"
                multiple={codeType === 'fixed'}
                className="hidden"
                onChange={(e) => void addFiles(Array.from(e.target.files ?? []))}
              />

              {drafts.length === 0 ? (
                <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed bg-muted/20 px-4 py-8 text-center">
                  <div className="flex size-11 items-center justify-center rounded-full bg-muted">
                    <ImagePlus className="size-5 text-muted-foreground" />
                  </div>
                  <div className="space-y-0.5">
                    <div className="text-sm font-medium">
                      {decoding ? '识别中…' : '拖入、选择图片，或按 Ctrl/⌘ + V 粘贴'}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {codeType === 'fixed' ? '固额码支持一次多张' : '任意金额码仅 1 张'}
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <Button
                      type="button"
                      className="h-9"
                      disabled={decoding || busy}
                      onClick={() => fileRef.current?.click()}
                    >
                      选择图片
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="space-y-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="mr-auto text-sm font-medium">
                      已选 {drafts.length} 张{decoding ? ' · 识别中…' : ''}
                    </span>
                    {codeType === 'fixed' && drafts.length > 1 ? (
                      <div className="flex items-center gap-1.5">
                        <Input
                          aria-label="批量设置金额"
                          className="h-9 w-full min-w-0 flex-1 text-sm sm:h-8 sm:w-36 sm:flex-none sm:text-xs"
                          inputMode="decimal"
                          value={bulkAmount}
                          onChange={(e) => setBulkAmount(e.target.value)}
                          placeholder="金额，如 1 2 5 10"
                          title="填 1 个 = 全部相同；填多个（空格/逗号分隔）= 按顺序分配"
                        />
                        <Button
                          type="button"
                          variant="secondary"
                          className="h-8 px-2.5 text-xs"
                          onClick={applyBulkAmount}
                        >
                          批量设置
                        </Button>
                      </div>
                    ) : null}
                    {codeType === 'fixed' ? (
                      <Button
                        type="button"
                        variant="outline"
                        className="h-8 gap-1 px-2.5 text-xs"
                        disabled={decoding || busy}
                        onClick={() => fileRef.current?.click()}
                      >
                        <Plus className="size-3.5" />
                        继续添加
                      </Button>
                    ) : null}
                  </div>

                  <div
                    className={cn(
                      'grid gap-2.5',
                      codeType === 'fixed'
                        ? 'grid-cols-3 sm:grid-cols-4'
                        : 'mx-auto w-40 grid-cols-1',
                    )}
                  >
                    {drafts.map((d, i) => (
                      <div
                        key={d.key}
                        className={cn(
                          'relative flex flex-col gap-1.5 rounded-lg border bg-card p-1.5',
                          d.error && 'border-destructive/60 bg-destructive/5',
                        )}
                      >
                        <button
                          type="button"
                          disabled={busy}
                          onClick={() => removeDraft(d.key)}
                          aria-label="移除"
                          className="absolute right-1 top-1 z-10 flex size-5 items-center justify-center rounded-full bg-foreground/70 text-background hover:bg-foreground"
                        >
                          <X className="size-3" />
                        </button>
                        <img
                          src={d.previewURL}
                          alt={`二维码 ${i + 1}`}
                          className="aspect-square w-full rounded-md bg-white object-contain"
                        />
                        {codeType === 'fixed' ? (
                          <div className="flex items-center gap-1">
                            <span className="pl-0.5 text-xs text-muted-foreground">¥</span>
                            <Input
                              className="h-8 min-w-0 flex-1 px-2 text-sm"
                              inputMode="decimal"
                              placeholder="金额"
                              value={d.amountYuan}
                              onChange={(e) => updateDraft(d.key, { amountYuan: e.target.value })}
                            />
                          </div>
                        ) : (
                          <div className="pb-0.5 text-center text-xs text-muted-foreground">
                            任意金额
                          </div>
                        )}
                        {d.error ? (
                          <div className="px-0.5 text-[11px] leading-tight text-destructive">
                            {d.error}
                          </div>
                        ) : null}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>

            <DialogFooter className="border-t bg-muted/20 px-5 py-3 sm:items-center">
              <Button type="button" variant="outline" onClick={() => setCodeOpen(false)}>
                取消
              </Button>
              <Button type="submit" disabled={busy || decoding || drafts.length === 0}>
                {busy ? '提交中…' : drafts.length > 1 ? `添加 ${drafts.length} 张` : '添加'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </Shell>
  )
}
