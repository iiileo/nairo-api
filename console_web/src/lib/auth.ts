const TK_KEY = 'nairo_console_tk'
const RK_KEY = 'nairo_console_rk'
const USER_KEY = 'nairo_console_user'

export type ConsoleUser = {
  id: string
  email?: string | null
  phone?: string | null
  fee_rate_bps: number
  adjust_direction: string
  adjust_enabled: boolean
}

export function getToken(): string | null {
  return localStorage.getItem(TK_KEY)
}

export function getRenew(): string | null {
  return localStorage.getItem(RK_KEY)
}

export function setSession(token: string, renew: string, user: ConsoleUser) {
  localStorage.setItem(TK_KEY, token)
  localStorage.setItem(RK_KEY, renew)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

export function clearSession() {
  localStorage.removeItem(TK_KEY)
  localStorage.removeItem(RK_KEY)
  localStorage.removeItem(USER_KEY)
}

export function getCachedUser(): ConsoleUser | null {
  const raw = localStorage.getItem(USER_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as ConsoleUser
  } catch {
    return null
  }
}
