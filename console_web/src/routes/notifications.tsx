import { createFileRoute } from '@tanstack/react-router'
import {
  ArrowLeft,
  Bell,
  BookOpen,
  CheckCircle2,
  ExternalLink,
  Loader2,
  Plus,
  Send,
  Trash2,
} from 'lucide-react'
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type Dispatch,
  type FormEvent,
  type ReactNode,
  type SetStateAction,
} from 'react'
import { toast } from 'sonner'
import { Shell } from '@/components/Shell'
import { ApiError, api, type AccountItem, type NotifyChannelItem } from '@/lib/api'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { requireAuth } from '@/lib/requireAuth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'
import { useConfirm } from '@/components/ConfirmDialog'

export const Route = createFileRoute('/notifications')({
  beforeLoad: () => requireAuth(),
  component: NotificationsPage,
})

type ProviderId = 'wecom' | 'feishu' | 'dingtalk'

type ProviderGuide = {
  id: ProviderId
  label: string
  short: string
  docUrl: string
  docTitle: string
  msgFormat: string
  webhookExample: string
  webhookHint: string
  webhookPrefix: string
  steps: string[]
  fields: Array<{
    key: 'webhook_url' | 'secret' | 'name'
    label: string
    required: boolean
    placeholder: string
    hint: string
  }>
  securityNotes: string[]
}

const PROVIDERS: ProviderGuide[] = [
  {
    id: 'wecom',
    label: '企业微信',
    short: '群机器人 Webhook',
    docUrl: 'https://developer.work.weixin.qq.com/document/path/91770',
    docTitle: '企业微信 · 群机器人配置说明',
    msgFormat: 'Markdown',
    webhookExample: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=693a…',
    webhookHint: '复制完整地址，需包含 key= 参数',
    webhookPrefix: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send',
    steps: [
      '打开企业微信群 → 右上角 … → 添加群机器人',
      '选择「自定义机器人」或「消息推送」并创建',
      '复制 Webhook 地址（含 key）粘贴到下方',
      '无需加签密钥；保存后点「测试」确认进群',
    ],
    fields: [
      {
        key: 'name',
        label: '备注名',
        required: false,
        placeholder: '例如：运营群',
        hint: '仅在控制台显示，方便区分多个机器人',
      },
      {
        key: 'webhook_url',
        label: 'Webhook URL',
        required: true,
        placeholder: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=…',
        hint: '必填。群机器人详情页复制的完整 URL',
      },
    ],
    securityNotes: [
      '企微群机器人不需要填写加签密钥',
      '请勿把 Webhook 发到公开仓库或聊天；泄露后他人可往群里发消息',
      '频率限制约 20 条/分钟',
    ],
  },
  {
    id: 'feishu',
    label: '飞书',
    short: '自定义机器人 · 卡片',
    docUrl: 'https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot',
    docTitle: '飞书 · 自定义机器人使用指南',
    msgFormat: '消息卡片（interactive）',
    webhookExample: 'https://open.feishu.cn/open-apis/bot/v2/hook/xxxxxxxx',
    webhookHint: '复制 hook 地址；若开启签名校验再填密钥',
    webhookPrefix: 'https://open.feishu.cn/open-apis/bot/v2/hook/',
    steps: [
      '打开飞书群 → 设置 → 群机器人 → 添加「自定义机器人」',
      '复制 Webhook 地址粘贴到下方',
      '若安全设置勾选了「签名校验」，把密钥填到「签名密钥」',
      '若只用「自定义关键词」，请包含「Nairo」（我们消息里已带）',
    ],
    fields: [
      {
        key: 'name',
        label: '备注名',
        required: false,
        placeholder: '例如：财务告警群',
        hint: '仅在控制台显示',
      },
      {
        key: 'webhook_url',
        label: 'Webhook URL',
        required: true,
        placeholder: 'https://open.feishu.cn/open-apis/bot/v2/hook/…',
        hint: '必填。添加机器人后复制的 hook 地址',
      },
      {
        key: 'secret',
        label: '签名密钥',
        required: false,
        placeholder: '开启签名校验时填写',
        hint: '选填。仅当机器人安全设置启用「签名校验」时需要',
      },
    ],
    securityNotes: [
      'Nairo 使用飞书卡片（msg_type=interactive）推送',
      '若开启签名校验，请填写密钥，系统会按飞书要求自动加签',
      '也可改用关键词 / IP 白名单；关键词请包含 Nairo',
    ],
  },
  {
    id: 'dingtalk',
    label: '钉钉',
    short: '自定义机器人 · 加签',
    docUrl: 'https://open.dingtalk.com/document/orgapp/custom-robot-access',
    docTitle: '钉钉 · 自定义机器人接入',
    msgFormat: 'Markdown',
    webhookExample: 'https://oapi.dingtalk.com/robot/send?access_token=XXXXXX',
    webhookHint: '复制完整 URL；加签请再填 SEC 开头密钥',
    webhookPrefix: 'https://oapi.dingtalk.com/robot/send',
    steps: [
      '打开钉钉群 → 群设置 → 智能群助手 → 添加「自定义」机器人',
      '安全设置建议选「加签」，复制 Webhook 与 SEC 密钥',
      '把 Webhook 与密钥分别填到下方对应项',
      '若选「自定义关键词」，关键词需包含「Nairo」',
    ],
    fields: [
      {
        key: 'name',
        label: '备注名',
        required: false,
        placeholder: '例如：值班群',
        hint: '仅在控制台显示',
      },
      {
        key: 'webhook_url',
        label: 'Webhook URL',
        required: true,
        placeholder: 'https://oapi.dingtalk.com/robot/send?access_token=…',
        hint: '必填。含 access_token 的完整地址',
      },
      {
        key: 'secret',
        label: '加签密钥 SEC',
        required: false,
        placeholder: 'SECxxxxxxxx',
        hint: '选填但推荐。安全设置为「加签」时必填，通常以 SEC 开头',
      },
    ],
    securityNotes: [
      '加签时服务端会自动附加 timestamp 与 sign',
      '每个机器人约 20 条/分钟，超限会限流约 10 分钟',
      '关键词模式下，消息正文需命中至少一个关键词',
    ],
  },
]

type Draft = {
  provider: ProviderId
  name: string
  webhook_url: string
  secret: string
  clearSecret: boolean
  enabled: boolean
  event_order_paid: boolean
  event_fee_low: boolean
  event_app_push: boolean
  event_device_offline: boolean
  /** '' = 全部设备（商户级） */
  account_id: string
}

const emptyDraft = (provider: ProviderId = 'feishu'): Draft => ({
  provider,
  name: '',
  webhook_url: '',
  secret: '',
  clearSecret: false,
  enabled: true,
  event_order_paid: true,
  event_fee_low: true,
  event_app_push: true,
  event_device_offline: true,
  account_id: '',
})

type Mode = 'list' | 'pick' | 'form'

function guideOf(id: string): ProviderGuide {
  return PROVIDERS.find((p) => p.id === id) ?? PROVIDERS[1]!
}

function validateWebhook(provider: ProviderId, url: string): string | null {
  const u = url.trim()
  if (!u.startsWith('http://') && !u.startsWith('https://')) {
    return 'Webhook 必须以 http:// 或 https:// 开头'
  }
  if (provider === 'wecom' && !u.includes('qyapi.weixin.qq.com')) {
    return '企业微信地址通常包含 qyapi.weixin.qq.com'
  }
  if (provider === 'feishu' && !u.includes('feishu.cn') && !u.includes('larksuite.com')) {
    return '飞书地址通常包含 open.feishu.cn（国际版可能为 larksuite.com）'
  }
  if (provider === 'dingtalk' && !u.includes('dingtalk.com')) {
    return '钉钉地址通常包含 oapi.dingtalk.com'
  }
  if (provider === 'wecom' && !u.includes('key=')) {
    return '企业微信 Webhook 需包含 key= 参数'
  }
  if (provider === 'dingtalk' && !u.includes('access_token=')) {
    return '钉钉 Webhook 需包含 access_token= 参数'
  }
  if (provider === 'feishu' && !u.includes('/hook/')) {
    return '飞书 Webhook 通常形如 …/bot/v2/hook/…'
  }
  return null
}

function NotificationsPage() {
  const confirm = useConfirm()
  const [items, setItems] = useState<NotifyChannelItem[]>([])
  const [accounts, setAccounts] = useState<AccountItem[]>([])
  const [loading, setLoading] = useState(true)
  const [mode, setMode] = useState<Mode>('list')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [editingHasSecret, setEditingHasSecret] = useState(false)
  const [draft, setDraft] = useState<Draft>(emptyDraft())
  const [busy, setBusy] = useState(false)
  const [testingId, setTestingId] = useState<string | null>(null)

  const guide = useMemo(() => guideOf(draft.provider), [draft.provider])
  const accountNameById = useMemo(() => {
    const m = new Map<string, string>()
    for (const a of accounts) {
      m.set(a.id, a.name || a.callback_id || a.id)
    }
    return m
  }, [accounts])

  const load = useCallback(async () => {
    try {
      const [chRes, accRes] = await Promise.all([api.notifyChannels(), api.accounts()])
      setItems(chRes.items)
      setAccounts(accRes.items)
    } catch {
      toast.error('加载通知配置失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  function openCreate() {
    setEditingId(null)
    setEditingHasSecret(false)
    setDraft(emptyDraft())
    setMode('pick')
  }

  function pickProvider(id: ProviderId) {
    setDraft(emptyDraft(id))
    setMode('form')
  }

  function openEdit(ch: NotifyChannelItem) {
    setEditingId(ch.id)
    setEditingHasSecret(ch.has_secret)
    setDraft({
      provider: (ch.provider as ProviderId) || 'feishu',
      name: ch.name,
      webhook_url: ch.webhook_url,
      secret: '',
      clearSecret: false,
      enabled: ch.enabled,
      event_order_paid: ch.event_order_paid,
      event_fee_low: ch.event_fee_low,
      event_app_push: ch.event_app_push,
      event_device_offline: ch.event_device_offline ?? true,
      account_id: ch.account_id || '',
    })
    setMode('form')
  }

  function backToList() {
    setMode('list')
    setEditingId(null)
    setDraft(emptyDraft())
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    const whErr = validateWebhook(draft.provider, draft.webhook_url)
    if (whErr) {
      toast.error(whErr)
      return
    }
    if (draft.provider === 'dingtalk' && draft.secret.trim() && !draft.secret.trim().startsWith('SEC')) {
      toast.message('提示：钉钉加签密钥通常以 SEC 开头，请确认已复制正确')
    }
    setBusy(true)
    try {
      const deviceBound = !!draft.account_id
      const base = {
        provider: draft.provider,
        name: draft.name.trim(),
        webhook_url: draft.webhook_url.trim(),
        enabled: draft.enabled,
        event_order_paid: draft.event_order_paid,
        event_fee_low: deviceBound ? false : draft.event_fee_low,
        event_app_push: draft.event_app_push,
        event_device_offline: draft.event_device_offline,
        account_id: draft.account_id || '',
      }
      if (editingId) {
        const payload: Parameters<typeof api.updateNotifyChannel>[1] = { ...base }
        if (draft.clearSecret) payload.secret = ''
        else if (draft.secret.trim()) payload.secret = draft.secret.trim()
        await api.updateNotifyChannel(editingId, payload)
        toast.success('已更新')
      } else {
        await api.createNotifyChannel({
          ...base,
          secret: draft.secret.trim() || undefined,
        })
        toast.success('已添加，建议立刻点「测试」')
      }
      backToList()
      await load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.code : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  async function onDelete(id: string) {
    const ok = await confirm({
      title: '删除通知机器人',
      description: '确定删除该通知机器人？',
      confirmText: '删除',
      destructive: true,
    })
    if (!ok) return
    try {
      await api.deleteNotifyChannel(id)
      toast.success('已删除')
      await load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.code : '删除失败')
    }
  }

  async function onTest(id: string) {
    setTestingId(id)
    try {
      await api.testNotifyChannel(id)
      toast.success('测试消息已发送，请到群里查看')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.detail || err.code : '发送失败')
    } finally {
      setTestingId(null)
    }
  }

  async function onToggle(ch: NotifyChannelItem, enabled: boolean) {
    try {
      await api.updateNotifyChannel(ch.id, {
        name: ch.name,
        webhook_url: ch.webhook_url,
        enabled,
        event_order_paid: ch.event_order_paid,
        event_fee_low: ch.event_fee_low,
        event_app_push: ch.event_app_push,
        event_device_offline: ch.event_device_offline,
        account_id: ch.account_id || '',
      })
      setItems((prev) => prev.map((x) => (x.id === ch.id ? { ...x, enabled } : x)))
    } catch (err) {
      toast.error(err instanceof ApiError ? err.code : '更新失败')
    }
  }

  return (
    <Shell>
      <div className="mb-5 flex flex-col gap-3 sm:mb-7 sm:flex-row sm:items-end sm:justify-between">
        <div className="space-y-1">
          <h1 className="font-display text-2xl font-bold tracking-tight sm:text-3xl">通知</h1>
          <p className="text-sm text-muted-foreground">
            按平台文档填写 Webhook；可绑定到单个监控设备，也可给全部设备共用。
          </p>
        </div>
        {mode === 'list' ? (
          <Button type="button" className="h-11 shrink-0 sm:h-9" onClick={openCreate}>
            <Plus className="size-4" />
            添加机器人
          </Button>
        ) : (
          <Button type="button" variant="outline" className="h-11 shrink-0 sm:h-9" onClick={backToList}>
            <ArrowLeft className="size-4" />
            返回列表
          </Button>
        )}
      </div>

      {mode === 'pick' ? <ProviderPicker onPick={pickProvider} /> : null}

      {mode === 'form' ? (
        <NotifyForm
          guide={guide}
          draft={draft}
          setDraft={setDraft}
          accounts={accounts}
          editing={!!editingId}
          editingHasSecret={editingHasSecret}
          busy={busy}
          onSubmit={onSubmit}
          onBack={() => (editingId ? backToList() : setMode('pick'))}
        />
      ) : null}

      {mode === 'list' ? (
        loading ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="size-4 animate-spin" />
            加载中…
          </div>
        ) : items.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col gap-3 py-4 sm:flex-row sm:items-center sm:justify-between sm:gap-4 sm:py-5">
              <div className="flex min-w-0 items-start gap-3">
                <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                  <Bell className="size-4" />
                </div>
                <div className="min-w-0">
                  <div className="font-display text-base font-semibold">尚未配置通知</div>
                  <p className="mt-0.5 text-sm text-muted-foreground">
                    先选平台，再按文档复制 Webhook（与加签密钥）。
                  </p>
                </div>
              </div>
              <Button type="button" className="h-10 shrink-0 sm:h-9" onClick={openCreate}>
                <Plus className="size-4" />
                添加机器人
              </Button>
            </CardContent>
          </Card>
        ) : (
          <div className="grid gap-3">
            {items.map((ch) => (
              <Card key={ch.id} className={cn(!ch.enabled && 'opacity-70')}>
                <CardContent className="flex flex-col gap-4 py-4 sm:flex-row sm:items-start sm:justify-between">
                  <div className="min-w-0 space-y-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="rounded-md bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">
                        {ch.provider_label}
                      </span>
                      <span className="font-medium">{ch.name || '未命名'}</span>
                      <span className="rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                        {ch.account_id
                          ? `设备 · ${accountNameById.get(ch.account_id) || ch.account_id}`
                          : '全部设备'}
                      </span>
                      {ch.has_secret ? (
                        <span className="text-xs text-muted-foreground">已加签</span>
                      ) : null}
                    </div>
                    <div className="truncate font-mono text-xs text-muted-foreground">{ch.webhook_url}</div>
                    <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                      {ch.event_order_paid ? <span>订单入账</span> : null}
                      {ch.event_fee_low ? <span>手续费不足</span> : null}
                      {ch.event_app_push ? <span>App 推送</span> : null}
                      {ch.event_device_offline ? <span>设备离线</span> : null}
                    </div>
                  </div>
                  <div className="flex flex-wrap items-center gap-2 sm:justify-end">
                    <Switch checked={ch.enabled} onCheckedChange={(v) => void onToggle(ch, v)} />
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-10 sm:h-8"
                      disabled={testingId === ch.id}
                      onClick={() => void onTest(ch.id)}
                    >
                      {testingId === ch.id ? (
                        <Loader2 className="size-3.5 animate-spin" />
                      ) : (
                        <Send className="size-3.5" />
                      )}
                      测试
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-10 sm:h-8"
                      onClick={() => openEdit(ch)}
                    >
                      编辑
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-10 sm:h-8 text-destructive"
                      onClick={() => void onDelete(ch.id)}
                    >
                      <Trash2 className="size-3.5" />
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )
      ) : null}
    </Shell>
  )
}

function ProviderPicker({ onPick }: { onPick: (id: ProviderId) => void }) {
  return (
    <div className="space-y-4">
      <div>
        <h2 className="font-display text-lg font-semibold">选择平台</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          不同平台参数不同：先选平台，再按文档指引填写。
        </p>
      </div>
      <div className="grid gap-3 sm:grid-cols-3">
        {PROVIDERS.map((p) => (
          <button
            key={p.id}
            type="button"
            onClick={() => onPick(p.id)}
            className="rounded-xl border bg-card p-4 text-left transition-colors hover:border-primary/40 hover:bg-accent/40"
          >
            <div className="font-display text-base font-semibold">{p.label}</div>
            <div className="mt-1 text-xs text-muted-foreground">{p.short}</div>
            <div className="mt-3 text-xs text-muted-foreground">消息格式 · {p.msgFormat}</div>
            <div className="mt-3 text-sm font-medium text-primary">下一步 →</div>
          </button>
        ))}
      </div>
    </div>
  )
}

function NotifyForm({
  guide,
  draft,
  setDraft,
  accounts,
  editing,
  editingHasSecret,
  busy,
  onSubmit,
  onBack,
}: {
  guide: ProviderGuide
  draft: Draft
  setDraft: Dispatch<SetStateAction<Draft>>
  accounts: AccountItem[]
  editing: boolean
  editingHasSecret: boolean
  busy: boolean
  onSubmit: (e: FormEvent) => void
  onBack: () => void
}) {
  const showSecret = guide.fields.some((f) => f.key === 'secret')
  const deviceBound = !!draft.account_id

  return (
    <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(280px,340px)]">
      <Card>
        <CardHeader className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="rounded-md bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">
              {guide.label}
            </span>
            <span className="text-xs text-muted-foreground">{guide.msgFormat}</span>
          </div>
          <CardTitle>{editing ? '编辑机器人' : `填写 ${guide.label} 参数`}</CardTitle>
          <CardDescription>
            按右侧说明复制参数。可绑定到单个设备，或给全部设备共用。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form className="space-y-5" onSubmit={onSubmit}>
            <FieldBlock
              label="适用设备"
              required={false}
              hint="选「全部设备」时，任意设备事件都会推到此机器人；选具体设备则仅该设备。"
            >
              <Select
                value={draft.account_id || 'all'}
                onValueChange={(v) =>
                  setDraft((d) => ({
                    ...d,
                    account_id: !v || v === 'all' ? '' : v,
                    event_fee_low: !v || v === 'all' ? d.event_fee_low : false,
                  }))
                }
              >
                <SelectTrigger className="h-11 w-full sm:h-9">
                  <SelectValue placeholder="选择设备" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部设备（商户级）</SelectItem>
                  {accounts.map((a) => (
                    <SelectItem key={a.id} value={a.id}>
                      {a.name || a.callback_id || a.id}
                      {a.channel_label ? ` · ${a.channel_label}` : ''}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </FieldBlock>

            {guide.fields.map((field) => {
              if (field.key === 'name') {
                return (
                  <FieldBlock key={field.key} label={field.label} required={field.required} hint={field.hint}>
                    <Input
                      value={draft.name}
                      onChange={(e) => setDraft((d) => ({ ...d, name: e.target.value }))}
                      placeholder={field.placeholder}
                    />
                  </FieldBlock>
                )
              }
              if (field.key === 'webhook_url') {
                return (
                  <FieldBlock key={field.key} label={field.label} required={field.required} hint={field.hint}>
                    <Input
                      value={draft.webhook_url}
                      onChange={(e) => setDraft((d) => ({ ...d, webhook_url: e.target.value }))}
                      placeholder={field.placeholder}
                      autoComplete="off"
                      spellCheck={false}
                      className="font-mono text-sm"
                    />
                    <p className="mt-1.5 font-mono text-[11px] leading-relaxed text-muted-foreground">
                      示例：{guide.webhookExample}
                    </p>
                  </FieldBlock>
                )
              }
              return null
            })}

            {showSecret ? (
              <FieldBlock
                label={guide.fields.find((f) => f.key === 'secret')?.label || '加签密钥'}
                required={false}
                hint={
                  editing
                    ? editingHasSecret
                      ? '已配置密钥。留空表示不修改；勾选下方可清空。'
                      : '当前未配置密钥。需要加签时填写。'
                    : guide.fields.find((f) => f.key === 'secret')?.hint
                }
              >
                <Input
                  type="password"
                  value={draft.secret}
                  onChange={(e) =>
                    setDraft((d) => ({ ...d, secret: e.target.value, clearSecret: false }))
                  }
                  placeholder={guide.fields.find((f) => f.key === 'secret')?.placeholder}
                  autoComplete="new-password"
                  className="font-mono text-sm"
                  disabled={draft.clearSecret}
                />
                {editing && editingHasSecret ? (
                  <label className="mt-2 flex items-center gap-2 text-sm text-muted-foreground">
                    <input
                      type="checkbox"
                      checked={draft.clearSecret}
                      onChange={(e) =>
                        setDraft((d) => ({
                          ...d,
                          clearSecret: e.target.checked,
                          secret: e.target.checked ? '' : d.secret,
                        }))
                      }
                    />
                    清空已保存的加签密钥
                  </label>
                ) : null}
              </FieldBlock>
            ) : (
              <div className="rounded-lg border border-dashed px-3 py-2.5 text-sm text-muted-foreground">
                {guide.label}群机器人<strong className="text-foreground">不需要</strong>
                加签密钥，只需 Webhook URL。
              </div>
            )}

            <div className="space-y-3 rounded-lg border p-3">
              <div className="text-sm font-medium">订阅事件</div>
              <EventSwitch
                label="订单入账"
                desc="自动匹配或人工补单成功"
                checked={draft.event_order_paid}
                onChange={(v) => setDraft((d) => ({ ...d, event_order_paid: v }))}
              />
              {deviceBound ? (
                <p className="text-xs text-muted-foreground">
                  设备专属通道不接收「手续费不足」（属商户级事件，请用「全部设备」通道订阅）。
                </p>
              ) : (
                <EventSwitch
                  label="手续费不足"
                  desc="Open API 下单因余额不足失败"
                  checked={draft.event_fee_low}
                  onChange={(v) => setDraft((d) => ({ ...d, event_fee_low: v }))}
                />
              )}
              <EventSwitch
                label="App 支付推送"
                desc="收到 App 上报（含未匹配到账）"
                checked={draft.event_app_push}
                onChange={(v) => setDraft((d) => ({ ...d, event_app_push: v }))}
              />
              <EventSwitch
                label="设备心跳丢失"
                desc="连续超过 3 次（约 30 秒）未收到 App 心跳"
                checked={draft.event_device_offline}
                onChange={(v) => setDraft((d) => ({ ...d, event_device_offline: v }))}
              />
            </div>

            <div className="flex items-center justify-between gap-3 rounded-lg border px-3 py-3">
              <div>
                <div className="font-medium">启用</div>
                <div className="text-sm text-muted-foreground">关闭后暂不推送</div>
              </div>
              <Switch
                checked={draft.enabled}
                onCheckedChange={(v) => setDraft((d) => ({ ...d, enabled: v }))}
              />
            </div>

            <div className="flex flex-wrap gap-2">
              <Button type="submit" disabled={busy} className="h-11 sm:h-9">
                {busy ? <Loader2 className="size-4 animate-spin" /> : null}
                保存
              </Button>
              <Button type="button" variant="outline" className="h-11 sm:h-9" onClick={onBack}>
                {editing ? '取消' : '重选平台'}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <aside className="space-y-3 lg:sticky lg:top-4 lg:self-start">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              <BookOpen className="size-4" />
              填写指引
            </CardTitle>
            <CardDescription>{guide.docTitle}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <ol className="space-y-2.5">
              {guide.steps.map((step, i) => (
                <li key={step} className="flex gap-2.5 text-sm leading-relaxed">
                  <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
                    {i + 1}
                  </span>
                  <span>{step}</span>
                </li>
              ))}
            </ol>

            <div className="space-y-2 rounded-lg bg-muted/50 p-3">
              <div className="text-xs font-medium text-muted-foreground">本页需要填写</div>
              <ul className="space-y-1.5">
                {guide.fields.map((f) => (
                  <li key={f.key} className="flex items-start gap-2 text-sm">
                    <CheckCircle2
                      className={cn(
                        'mt-0.5 size-3.5 shrink-0',
                        f.required ? 'text-primary' : 'text-muted-foreground',
                      )}
                    />
                    <span>
                      <span className="font-medium">{f.label}</span>
                      <span className="text-muted-foreground">
                        {f.required ? ' · 必填' : ' · 选填'}
                      </span>
                    </span>
                  </li>
                ))}
              </ul>
            </div>

            <ul className="space-y-1.5 text-xs leading-relaxed text-muted-foreground">
              {guide.securityNotes.map((n) => (
                <li key={n}>· {n}</li>
              ))}
            </ul>

            <a
              href={guide.docUrl}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline"
            >
              打开官方文档
              <ExternalLink className="size-3.5" />
            </a>
          </CardContent>
        </Card>
      </aside>
    </div>
  )
}

function FieldBlock({
  label,
  required,
  hint,
  children,
}: {
  label: string
  required: boolean
  hint?: string
  children: ReactNode
}) {
  return (
    <div className="space-y-2">
      <div className="flex items-baseline gap-2">
        <Label>
          {label}
          {required ? <span className="text-destructive"> *</span> : null}
        </Label>
        {!required ? <span className="text-xs text-muted-foreground">选填</span> : null}
      </div>
      {children}
      {hint ? <p className="text-xs leading-relaxed text-muted-foreground">{hint}</p> : null}
    </div>
  )
}

function EventSwitch({
  label,
  desc,
  checked,
  onChange,
}: {
  label: string
  desc: string
  checked: boolean
  onChange: (v: boolean) => void
}) {
  return (
    <div className="flex items-center justify-between gap-3">
      <div>
        <div className="text-sm font-medium">{label}</div>
        <div className="text-xs text-muted-foreground">{desc}</div>
      </div>
      <Switch checked={checked} onCheckedChange={onChange} />
    </div>
  )
}
