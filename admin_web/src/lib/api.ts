import { clearSession, getToken } from './auth'

export class ApiError extends Error {
  status: number
  code: string
  constructor(status: number, code: string) {
    super(code)
    this.status = status
    this.code = code
  }
}

async function request<T>(method: string, path: string, body?: unknown, auth = true): Promise<T> {
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
  if (!res.ok) {
    if (res.status === 401 && auth) clearSession()
    throw new ApiError(res.status, data?.error || `http_${res.status}`)
  }
  return data as T
}

export type LoginResult = {
  token: string
  admin: { id: string; username: string; name?: string }
}

export type UserItem = {
  id: string
  email?: string | null
  phone?: string | null
  fee_rate_bps: number
  status: string
  adjust_direction: string
}

export type OrderItem = {
  id: string
  user_id?: string
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

export type AmountPattern = {
  name: string
  regex: string
  group: number
}

export type AppMatchRules = {
  version: number
  updated_at?: string
  watch_packages: Record<string, string>
  receive_keywords: string[]
  outbound_keywords: string[]
  ignore_keywords: string[]
  sanitize_patterns: string[]
  amount_patterns: AmountPattern[]
}

export type RuntimeConfigView = {
  source: string
  smtp: {
    host: string
    port: number
    user: string
    from: string
    dev_print: boolean
    password_set: boolean
  }
  pay: {
    public_base_url: string
    lock_ttl_seconds: number
  }
  console: {
    default_fee_rate_bps: number
    code_ttl_seconds: number
    register_enabled: boolean
  }
  geetest: {
    captcha_id: string
    key_set: boolean
  }
}

export type RuntimeConfigPatch = {
  smtp?: {
    host?: string
    port?: number
    user?: string
    password?: string
    from?: string
    dev_print?: boolean
  }
  pay?: {
    public_base_url?: string
    lock_ttl_seconds?: number
  }
  console?: {
    default_fee_rate_bps?: number
    code_ttl_seconds?: number
    register_enabled?: boolean
  }
  geetest?: {
    captcha_id?: string
    captcha_key?: string
  }
}

export const api = {
  login: (username: string, password: string) =>
    request<LoginResult>('POST', '/api/v1/admin/login', { username, password }, false),
  users: (limit = 50, offset = 0) =>
    request<{ items: UserItem[] }>(`GET`, `/api/v1/admin/users?limit=${limit}&offset=${offset}`),
  createUser: (body: { email?: string; phone?: string; password: string; fee_rate_bps?: number }) =>
    request<{
      user: UserItem
      access_key: string
      api_secret: string
    }>('POST', '/api/v1/admin/users', body),
  updateUser: (
    id: string,
    body: { email?: string | null; phone?: string | null; password?: string; status?: string },
  ) =>
    request<{
      id: string
      email?: string | null
      phone?: string | null
      status: string
      fee_rate_bps: number
    }>('POST', `/api/v1/admin/users/${id}`, body),
  setPassword: (id: string, password: string) =>
    request<{ id: string; ok: boolean }>('POST', `/api/v1/admin/users/${id}/password`, { password }),
  setFee: (id: string, fee_rate_bps: number) =>
    request<{ id: string; fee_rate_bps: number }>('POST', `/api/v1/admin/users/${id}/fee-rate`, {
      fee_rate_bps,
    }),
  recharge: (id: string, amount_cents: number, remark?: string) =>
    request<{ balance_cents: number }>('POST', `/api/v1/admin/users/${id}/recharge`, {
      amount_cents,
      remark,
    }),
  platformSettings: () =>
    request<{
      pay_user_id: string
      pay_user_email?: string | null
      pay_user_phone?: string | null
      pay_user_ok: boolean
      hint: string
    }>('GET', '/api/v1/admin/platform-settings'),
  setPlatformPayUser: (user_id: string) =>
    request<{
      ok: boolean
      pay_user_id: string
      pay_user_ok: boolean
    }>('POST', '/api/v1/admin/platform-settings/pay-user', { user_id }),
  matchRules: () =>
    request<{ rules: AppMatchRules; hint: string }>('GET', '/api/v1/admin/platform-settings/match-rules'),
  setMatchRules: (rules: AppMatchRules) =>
    request<{ ok: boolean; rules: AppMatchRules }>('PUT', '/api/v1/admin/platform-settings/match-rules', rules),
  resetMatchRules: () =>
    request<{ ok: boolean; rules: AppMatchRules }>(
      'POST',
      '/api/v1/admin/platform-settings/match-rules/reset',
    ),
  runtimeConfig: () =>
    request<{ config: RuntimeConfigView; hint: string }>('GET', '/api/v1/admin/platform-settings/runtime'),
  setRuntimeConfig: (body: RuntimeConfigPatch) =>
    request<{ ok: boolean; config: RuntimeConfigView }>(
      'PUT',
      '/api/v1/admin/platform-settings/runtime',
      body,
    ),
  reloadRuntimeConfig: () =>
    request<{ ok: boolean; config: RuntimeConfigView }>(
      'POST',
      '/api/v1/admin/platform-settings/runtime/reload',
    ),
  createAccount: (body: { user_id: string; channels?: string[]; channel?: string; name?: string }) =>
    request<any>('POST', '/api/v1/admin/accounts', body),
  createPayCode: (body: {
    account_id: string
    channel: string
    code_type?: string
    amount_cents?: number
    image_url?: string
    payload?: string
  }) => request<any>('POST', '/api/v1/admin/pay-codes', body),
  orders: (status?: string) =>
    request<{ items: OrderItem[] }>(
      'GET',
      `/api/v1/admin/orders${status ? `?status=${encodeURIComponent(status)}` : ''}`,
    ),
  manualPay: (id: string, body: { remark: string; channel?: string; pay_amount?: string }) =>
    request<{ order_id: string; status: string }>('POST', `/api/v1/admin/orders/${id}/manual-pay`, body),
  setOrderStatus: (
    id: string,
    body: { status: 'paid' | 'closed' | 'expired'; remark: string; channel?: string; pay_amount?: string },
  ) =>
    request<{ order_id: string; status: string }>('POST', `/api/v1/admin/orders/${id}/status`, body),
  orphans: () => request<{ items: OrphanItem[] }>('GET', '/api/v1/admin/orphans'),
  bindOrphan: (id: string, body: { order_id: string; remark: string }) =>
    request<{ order_id: string; status: string }>('POST', `/api/v1/admin/orphans/${id}/bind`, body),
  ignoreOrphan: (id: string, remark?: string) =>
    request<{ ok: boolean; id: string }>('POST', `/api/v1/admin/orphans/${id}/ignore`, {
      remark: remark || 'ignored',
    }),
}
