export type GeetestProof = {
  lot_number: string
  captcha_output: string
  pass_token: string
  gen_time: string
}

type GeetestValidate = {
  lot_number: string
  captcha_output: string
  pass_token: string
  gen_time: string
}

type GeetestCaptcha = {
  onReady: (cb: () => void) => GeetestCaptcha
  onSuccess: (cb: () => void) => GeetestCaptcha
  onError: (cb: (err?: { code?: string; msg?: string }) => void) => GeetestCaptcha
  onClose: (cb: () => void) => GeetestCaptcha
  onFail: (cb: (fail?: unknown) => void) => GeetestCaptcha
  showCaptcha: () => void
  reset?: () => void
  getValidate: () => GeetestValidate | false
  destroy?: () => void
}

type InitConfig = {
  captchaId: string
  product?: string
  language?: string
  protocol?: string
  timeout?: number
  hideSuccess?: boolean
  apiServers?: string[]
  onError?: (err?: { code?: string; msg?: string }) => void
}

declare global {
  interface Window {
    initGeetest4?: (config: InitConfig, callback: (captcha: GeetestCaptcha) => void) => void
  }
}

const SCRIPT_URLS = [
  'https://static.geetest.com/v4/gt4.js',
  'https://static.geevisit.com/v4/gt4.js',
]

let loading: Promise<void> | null = null

function loadGt4Script(): Promise<void> {
  if (typeof window === 'undefined') return Promise.reject(new Error('no window'))
  if (window.initGeetest4) return Promise.resolve()
  if (loading) return loading

  loading = (async () => {
    let lastErr: Error | null = null
    for (const src of SCRIPT_URLS) {
      try {
        await new Promise<void>((resolve, reject) => {
          const s = document.createElement('script')
          s.src = src
          s.async = true
          s.onload = () => resolve()
          s.onerror = () => reject(new Error(`load failed: ${src}`))
          document.head.appendChild(s)
        })
        if (window.initGeetest4) return
      } catch (e) {
        lastErr = e instanceof Error ? e : new Error(String(e))
      }
    }
    loading = null
    throw lastErr || new Error('geetest script load failed')
  })()

  return loading
}

export type GeetestSession = {
  show: () => Promise<GeetestProof>
  destroy: () => void
}

function errMsg(err?: { code?: string; msg?: string } | Error | string | null): string {
  if (!err) return '人机验证加载失败'
  if (typeof err === 'string') return err
  if (err instanceof Error) return err.message || '人机验证失败'
  if (err.msg) return err.msg
  if (err.code) return `验证失败(${err.code})`
  return '人机验证失败'
}

/**
 * 页面加载时初始化（GT4 要求尽早 init，否则行为数据为空且易报网络错误）。
 * 业务按钮再调用 session.show()。
 */
export async function createGeetestSession(captchaId: string): Promise<GeetestSession> {
  await loadGt4Script()
  if (!window.initGeetest4) {
    throw new Error('initGeetest4 unavailable')
  }

  let captcha: GeetestCaptcha | null = null
  let ready = false
  let readyWaiters: Array<() => void> = []
  let pending:
    | {
        resolve: (v: GeetestProof) => void
        reject: (e: Error) => void
      }
    | null = null

  const failPending = (e: Error) => {
    if (!pending) return
    const p = pending
    pending = null
    p.reject(e)
  }

  await new Promise<void>((resolve, reject) => {
    let settled = false
    const ok = () => {
      if (settled) return
      settled = true
      resolve()
    }
    const bad = (e: Error) => {
      if (settled) return
      settled = true
      reject(e)
    }

    try {
      window.initGeetest4!(
        {
          captchaId,
          product: 'bind',
          language: 'zho',
          protocol: 'https://',
          timeout: 15000,
          hideSuccess: true,
          apiServers: ['gcaptcha4.geetest.com', 'gcaptcha4.geevisit.com', 'gcaptcha4.gsensebot.com'],
          onError: (err) => {
            const msg = errMsg(err)
            failPending(new Error(msg))
            bad(new Error(msg))
          },
        },
        (obj) => {
          captcha = obj
          obj
            .onReady(() => {
              ready = true
              readyWaiters.splice(0).forEach((fn) => fn())
              ok()
            })
            .onSuccess(() => {
              const v = obj.getValidate()
              if (!v || !v.lot_number) {
                failPending(new Error('empty validate'))
                return
              }
              if (!pending) return
              const p = pending
              pending = null
              p.resolve({
                lot_number: v.lot_number,
                captcha_output: v.captcha_output,
                pass_token: v.pass_token,
                gen_time: v.gen_time,
              })
            })
            .onError((err) => {
              failPending(new Error(errMsg(err)))
              try {
                obj.reset?.()
              } catch {
                /* ignore */
              }
            })
            .onClose(() => {
              failPending(new Error('请完成人机验证'))
            })
            .onFail(() => {
              // 用户操作失败，保持弹层可重试，不结束 Promise
            })
        },
      )
    } catch (e) {
      bad(e instanceof Error ? e : new Error(errMsg(e as Error)))
    }
  })

  return {
    show: () =>
      new Promise<GeetestProof>((resolve, reject) => {
        if (!captcha) {
          reject(new Error('captcha not ready'))
          return
        }
        if (pending) {
          pending.reject(new Error('cancelled'))
        }
        pending = { resolve, reject }
        const run = () => {
          try {
            captcha!.reset?.()
            captcha!.showCaptcha()
          } catch (e) {
            failPending(e instanceof Error ? e : new Error(errMsg(e as Error)))
          }
        }
        if (ready) run()
        else readyWaiters.push(run)
      }),
    destroy: () => {
      failPending(new Error('destroyed'))
      try {
        captcha?.destroy?.()
      } catch {
        /* ignore */
      }
      captcha = null
    },
  }
}
