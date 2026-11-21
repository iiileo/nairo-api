import { clearSession, getRenew, getToken, setSession, type ConsoleUser } from './auth'

export class ApiError extends Error {
  status: number
  code: string
  detail?: string
  constructor(status: number, code: string, detail?: string) {
    super(code)
    this.status = status
    this.code = code
    this.detail = detail
  }
}

export type TokenBundle = {
  token: string
  renew: string
  expires_in: number
  renew_in: number
  user: ConsoleUser
}

let refreshPromise: Promise<boolean> | null = null

async function tryRefresh(): Promise<boolean> {
  const renew = getRenew()
  if (!renew) return false
  try {
    const res = await fetch('/api/v1/console/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ renew }),
    })
    if (!res.ok) {
      clearSession()
      return false
    }
    const data = (await res.json()) as TokenBundle
    setSession(data.token, data.renew, data.user)
    return true
  } catch {
    clearSession()
    return false
  }
}

async function refreshOnce(): Promise<boolean> {
  if (!refreshPromise) {
    refreshPromise = tryRefresh().finally(() => {
      refreshPromise = null
    })
  }
  return refreshPromise
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  auth = true,
  retried = false,
): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (auth) {
    const token = getToken()
    if (token) headers.Authorization = `Bearer ${token}`
  }
  const res = await fetch(path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const text = await res.text()
  let data: any = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = { error: text }
    }
  }
  if (res.status === 401 && auth && !retried) {
    const ok = await refreshOnce()
    if (ok) return request<T>(method, path, body, auth, true)
    clearSession()
    throw new ApiError(401, data?.error || 'unauthorized')
  }
  if (!res.ok) {
    const detail =
      (typeof data?.message === 'string' && data.message) ||
      (typeof data?.detail === 'string' && data.detail) ||
      undefined
    throw new ApiError(res.status, data?.error || `http_${res.status}`, detail)
  }
  return data as T
}

export type ProfileResult = {
  user: ConsoleUser & {
    adjust_step_cents: number
    adjust_max_offset_cents: number
    console_last_seen_at?: string | null
  }
  wallet: { balance_cents: number }
}

export type DashboardResult = {
  user: ProfileResult['user']
  wallet: { balance_cents: number }
  online_accounts: number
  today_paid: { count: number; amount_cents: number }
}

export type OrderItem = {
  id: string
  out_trade_no: string
  amount_cents: number
  pay_amount_cents: number
  channel: string
  status: string
  settle_type: string
  paid_at?: string | null
}

export type OrphanItem = {
  id: string
  user_id: string
  account_id: string
  channel: string
  pay_amount_cents: number
  reported_at: string
  status: string
}

export type BatchManualResult = {
  ok: number
  fail: number
  items: { id: string; ok: boolean; error?: string }[]
}

export type AppBindConfig = {
  v: number
  name: string
  base_url: string
  push_url: string
  heartbeat_url: string
  heartbeat_ws_url?: string
  callback_id: string
  secret: string
  channels: string[]
}

export type AccountItem = {
  id: string
  channel: string
  channels?: string[]
  channel_label?: string
  name: string
  callback_id: string
  callback_url: string
  heartbeat_url?: string
  status: string
  app_last_seen_at?: string | null
  app_conn_type?: string | null
  online: boolean
  phone_bound?: boolean
  phone_name?: string
  phone_bound_at?: string | null
}

export type PayCodeItem = {
  id: string
  account_id: string
  channel: string
  code_type: string
  amount_cents?: number | null
  image_url: string
  payload?: string | null
  status: string
}

export type LedgerItem = {
  id: string
  type: string
  amount_cents: number
  balance_after_cents: number
  order_id?: string | null
  remark: string
  created_at: string
}

export type NotifyChannelItem = {
  id: string
  account_id?: string | null
  provider: string
  provider_label: string
  name: string
  webhook_url: string
  has_secret: boolean
  enabled: boolean
  event_order_paid: boolean
  event_fee_low: boolean
  event_app_push: boolean
  event_device_offline: boolean
  created_at: string
}

export type NotifyChannelWrite = {
  provider?: string
  name?: string
  webhook_url?: string
  secret?: string
  enabled?: boolean
  event_order_paid?: boolean
  event_fee_low?: boolean
  event_app_push?: boolean
  event_device_offline?: boolean
  /** 空字符串=商户级（全部设备）；设备 id=仅该设备 */
  account_id?: string | null
}

/** 与后端 PayCodeView 一致：展示二维码只用 display_url */
export type PayCodeView = {
  id: string
  channel?: string
  code_type: string
  image_url: string
  payload: string
  display_url: string
}

/** 与后端 PayOrderView 一致 */
export type WalletPayOrder = {
  order_id: string
  out_trade_no: string
  purpose?: string
  amount_cents: number
  pay_amount_cents: number
  adjust_offset_cents?: number
  channel: string
  status: string
  expire_at?: string | null
  expire_in?: number
  expired?: boolean
  paid_at?: string | null
  created_at?: string
  pay_page?: string
  pay_url?: string
  warning?: string
  /** @deprecated 使用 pay_code.display_url */
  qr_url?: string
  /** @deprecated 使用 pay_code.payload */
  qr_content?: string
  pay_code?: PayCodeView
}

export function payDisplayURL(order: Pick<WalletPayOrder, 'pay_code' | 'qr_url'>): string {
  return order.pay_code?.display_url || order.qr_url || ''
}

export function payPayload(order: Pick<WalletPayOrder, 'pay_code' | 'qr_content' | 'qr_url'>): string {
  const fromCode = order.pay_code?.payload?.trim()
  if (fromCode) return fromCode
  if (order.qr_content?.trim()) return order.qr_content.trim()
  const display = payDisplayURL(order)
  try {
    const url = new URL(display, typeof location !== 'undefined' ? location.origin : 'http://local')
    const data = url.searchParams.get('data')
    if (data) return data.trim()
  } catch {
    /* ignore */
  }
  return ''
}

/** 手机端唤起支付宝扫码收款（个人码 / qr.alipay.com） */
export function alipaySchemaURL(payload: string): string {
  const raw = payload.trim()
  if (!raw) return ''
  if (/^alipays?:\/\//i.test(raw) || /^alipayqr:\/\//i.test(raw)) return raw
  let qrcode = raw
  if (!/^https?:\/\//i.test(qrcode) && /qr\.alipay\.com/i.test(qrcode)) {
    qrcode = `https://${qrcode.replace(/^\/\//, '')}`
  }
  return `alipays://platformapi/startapp?saId=10000007&qrcode=${encodeURIComponent(qrcode)}`
}

export function isMobileUA(ua = typeof navigator !== 'undefined' ? navigator.userAgent : ''): boolean {
  return /Android|iPhone|iPad|iPod|Mobile|HarmonyOS/i.test(ua)
}

export const api = {
  login: (login: string, password: string) =>
    request<TokenBundle>('POST', '/api/v1/console/login', { login, password }, false),
  geetest: () =>
    request<{ enabled: boolean; captcha_id: string; register_enabled: boolean }>(
      'GET',
      '/api/v1/public/geetest',
      undefined,
      false,
    ),
  sendRegisterCode: (
    email: string,
    captcha?: {
      lot_number: string
      captcha_output: string
      pass_token: string
      gen_time: string
    },
  ) =>
    request<{ ok: boolean }>(
      'POST',
      '/api/v1/console/register/send-code',
      { email, ...(captcha || {}) },
      false,
    ),
  register: (body: {
    email: string
    code: string
    password: string
    confirm_password: string
  }) => request<TokenBundle>('POST', '/api/v1/console/register', body, false),
  logout: async () => {
    const renew = getRenew()
    try {
      if (renew) {
        await request<{ ok: boolean }>('POST', '/api/v1/console/logout', { renew }, false)
      }
    } finally {
      clearSession()
    }
  },
  me: () => request<ProfileResult>('GET', '/api/v1/console/me'),
  dashboard: () => request<DashboardResult>('GET', '/api/v1/console/dashboard'),
  heartbeat: () => request<{ ok: boolean }>('POST', '/api/v1/console/heartbeat', {}),
  updateAdjust: (body: {
    adjust_enabled?: boolean
    adjust_direction?: string
    adjust_step_cents?: number
    adjust_max_offset_cents?: number
  }) =>
    request<{
      adjust_enabled: boolean
      adjust_direction: string
      adjust_step_cents: number
      adjust_max_offset_cents: number
    }>('POST', '/api/v1/console/adjust', body),

  accounts: () => request<{ items: AccountItem[] }>('GET', '/api/v1/console/accounts'),
  createAccount: (body: { channels: string[]; name?: string }) =>
    request<{ account: AccountItem; app_secret: string; bind: AppBindConfig }>(
      'POST',
      '/api/v1/console/accounts',
      body,
    ),
  /** 不重置密钥，取当前扫码配置（再次扫码添加 App） */
  getBindConfig: (id: string) =>
    request<{ bind: AppBindConfig }>('GET', `/api/v1/console/accounts/${id}/bind-config`),
  resetAppSecret: (id: string) =>
    request<{ account_id: string; app_secret: string; callback_url: string; bind: AppBindConfig }>(
      'POST',
      `/api/v1/console/accounts/${id}/reset-app-secret`,
      {},
    ),
  updateChannels: (id: string, channels: string[]) =>
    request<{ id: string; channel: string; channels: string[]; channel_label: string }>(
      'POST',
      `/api/v1/console/accounts/${id}/channels`,
      { channels },
    ),
  deleteAccount: (id: string) =>
    request<{ ok: boolean; id: string }>('DELETE', `/api/v1/console/accounts/${id}`),
  releasePhone: (id: string) =>
    request<{ ok: boolean; id: string }>('POST', `/api/v1/console/accounts/${id}/release-phone`),
  payCodes: (accountId: string) =>
    request<{ items: PayCodeItem[] }>('GET', `/api/v1/console/accounts/${accountId}/pay-codes`),
  deletePayCode: (id: string) =>
    request<{ ok: boolean; id: string }>('DELETE', `/api/v1/console/pay-codes/${id}`),
  createPayCode: (body: {
    account_id: string
    channel: string
    code_type?: string
    amount_cents?: number
    image_url?: string
    payload?: string
  }) => request<PayCodeItem>('POST', '/api/v1/console/pay-codes', body),

  ledgers: (limit = 50) =>
    request<{ items: LedgerItem[] }>('GET', `/api/v1/console/wallet/ledgers?limit=${limit}`),
  walletPayStatus: () =>
    request<{
      configured: boolean
      is_platform_pay_user: boolean
      pay_user_id: string
      min_cents: number
      max_cents: number
      non_refundable: boolean
      warning: string
    }>('GET', '/api/v1/console/wallet/pay-status'),
  createWalletPay: (body: {
    amount_cents: number
    channel: string
    purpose?: 'recharge'
  }) => request<WalletPayOrder>('POST', '/api/v1/console/wallet/pay', body),
  getWalletPay: (id: string) => request<WalletPayOrder>('GET', `/api/v1/console/wallet/pay/${id}`),
  /** 商户自测下单：走自己的收款码，返回 pay_url 可跳转收银台 */
  createTestOrder: (body: { amount_cents: number; channel: string }) =>
    request<WalletPayOrder>('POST', '/api/v1/console/orders/test', body),
  credential: () =>
    request<{ merchant_id: string; access_key: string; has_secret: boolean; hint: string }>(
      'GET',
      '/api/v1/console/credential',
    ),
  resetCredential: () =>
    request<{ merchant_id: string; access_key: string; api_secret: string }>(
      'POST',
      '/api/v1/console/credential/reset',
      {},
    ),

  notifyChannels: () =>
    request<{ items: NotifyChannelItem[] }>('GET', '/api/v1/console/notify-channels'),
  createNotifyChannel: (body: NotifyChannelWrite) =>
    request<NotifyChannelItem>('POST', '/api/v1/console/notify-channels', body),
  updateNotifyChannel: (id: string, body: NotifyChannelWrite) =>
    request<NotifyChannelItem>('PUT', `/api/v1/console/notify-channels/${id}`, body),
  deleteNotifyChannel: (id: string) =>
    request<{ ok: boolean; id: string }>('DELETE', `/api/v1/console/notify-channels/${id}`),
  testNotifyChannel: (id: string) =>
    request<{ ok: boolean }>('POST', `/api/v1/console/notify-channels/${id}/test`, {}),

  orders: (params?: {
    status?: string
    channel?: string
    settle_type?: string
    q?: string
    limit?: number
  }) => {
    const qs = new URLSearchParams()
    if (params?.status && params.status !== 'all') qs.set('status', params.status)
    if (params?.channel && params.channel !== 'all') qs.set('channel', params.channel)
    if (params?.settle_type && params.settle_type !== 'all') qs.set('settle_type', params.settle_type)
    if (params?.q?.trim()) qs.set('q', params.q.trim())
    if (params?.limit) qs.set('limit', String(params.limit))
    const q = qs.toString()
    return request<{ items: OrderItem[] }>('GET', `/api/v1/console/orders${q ? `?${q}` : ''}`)
  },
  manualPay: (id: string, body: { remark: string; channel?: string; pay_amount?: string }) =>
    request<{ order_id: string; status: string; settle_type: string }>(
      'POST',
      `/api/v1/console/orders/${id}/manual-pay`,
      body,
    ),
  setOrderStatus: (
    id: string,
    body: { status: 'paid' | 'closed' | 'expired'; remark: string; channel?: string; pay_amount?: string },
  ) =>
    request<{ order_id: string; status: string; settle_type: string }>(
      'POST',
      `/api/v1/console/orders/${id}/status`,
      body,
    ),
  batchSetOrderStatus: (body: {
    ids: string[]
    status: 'paid' | 'closed' | 'expired'
    remark: string
    channel?: string
    pay_amount?: string
  }) => request<BatchManualResult>('POST', '/api/v1/console/orders/batch-status', body),
  orphans: (params?: { status?: string; channel?: string; limit?: number }) => {
    const qs = new URLSearchParams()
    if (params?.status) qs.set('status', params.status)
    if (params?.channel && params.channel !== 'all') qs.set('channel', params.channel)
    if (params?.limit) qs.set('limit', String(params.limit))
    const q = qs.toString()
    return request<{ items: OrphanItem[] }>('GET', `/api/v1/console/orphans${q ? `?${q}` : ''}`)
  },
  bindOrphan: (id: string, body: { order_id: string; remark: string }) =>
    request<{ order_id: string; status: string; settle_type: string }>(
      'POST',
      `/api/v1/console/orphans/${id}/bind`,
      body,
    ),
  ignoreOrphan: (id: string, remark?: string) =>
    request<{ ok: boolean; id: string }>('POST', `/api/v1/console/orphans/${id}/ignore`, {
      remark: remark || 'ignored',
    }),
  batchIgnoreOrphans: (body: { ids: string[]; remark?: string }) =>
    request<BatchManualResult>('POST', '/api/v1/console/orphans/batch-ignore', body),
}
