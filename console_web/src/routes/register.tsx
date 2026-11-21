import { Link, createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { BrandMark } from '@/components/BrandMark'
import { ApiError, api } from '@/lib/api'
import { getToken, setSession } from '@/lib/auth'
import { createGeetestSession, type GeetestSession } from '@/lib/geetest'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export const Route = createFileRoute('/register')({
  beforeLoad: () => {
    if (getToken()) throw redirect({ to: '/' })
  },
  component: RegisterPage,
})

const errMsg: Record<string, string> = {
  email_taken: '该邮箱已注册',
  rate_limited: '发送过于频繁，请稍后再试',
  code_invalid: '验证码无效或已过期',
  captcha_failed: '人机验证失败，请重试',
  register_disabled: '注册已关闭，请联系管理员',
  invalid_request: '请检查邮箱、密码与确认密码',
}

function RegisterPage() {
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [loading, setLoading] = useState(false)
  const [sending, setSending] = useState(false)
  const [cooldown, setCooldown] = useState(0)
  const [registerEnabled, setRegisterEnabled] = useState(true)
  const [configLoaded, setConfigLoaded] = useState(false)
  const [captchaEnabled, setCaptchaEnabled] = useState(false)
  const [captchaReady, setCaptchaReady] = useState(false)
  const [captchaBootError, setCaptchaBootError] = useState('')
  const sessionRef = useRef<GeetestSession | null>(null)

  useEffect(() => {
    if (cooldown <= 0) return
    const t = window.setTimeout(() => setCooldown((c) => c - 1), 1000)
    return () => window.clearTimeout(t)
  }, [cooldown])

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const res = await api.geetest()
        if (cancelled) return
        const open = res.register_enabled !== false
        setRegisterEnabled(open)
        setConfigLoaded(true)
        if (!open) {
          setCaptchaEnabled(false)
          setCaptchaReady(true)
          return
        }
        if (!res.enabled || !res.captcha_id) {
          setCaptchaEnabled(false)
          setCaptchaReady(true)
          return
        }
        setCaptchaEnabled(true)
        const session = await createGeetestSession(res.captcha_id)
        if (cancelled) {
          session.destroy()
          return
        }
        sessionRef.current = session
        setCaptchaReady(true)
      } catch (e) {
        if (cancelled) return
        setConfigLoaded(true)
        setCaptchaEnabled(true)
        setCaptchaReady(false)
        setCaptchaBootError(e instanceof Error ? e.message : '人机验证初始化失败')
      }
    })()
    return () => {
      cancelled = true
      sessionRef.current?.destroy()
      sessionRef.current = null
    }
  }, [])

  async function onSendCode() {
    if (!registerEnabled) {
      toast.error('注册已关闭，请联系管理员')
      return
    }
    if (!email.trim()) {
      toast.error('请先填写邮箱')
      return
    }
    if (captchaEnabled && !captchaReady) {
      toast.error(captchaBootError || '人机验证加载中，请稍候')
      return
    }
    setSending(true)
    try {
      let captcha:
        | {
            lot_number: string
            captcha_output: string
            pass_token: string
            gen_time: string
          }
        | undefined
      if (captchaEnabled) {
        const session = sessionRef.current
        if (!session) {
          toast.error(captchaBootError || '人机验证未就绪')
          return
        }
        try {
          captcha = await session.show()
        } catch (e) {
          const msg = e instanceof Error ? e.message : ''
          if (msg && msg !== '请完成人机验证' && msg !== 'cancelled') {
            toast.error(msg.includes('网络') ? '人机验证网络异常，请检查域名是否已在极验后台配置' : msg)
          } else {
            toast.error('请完成人机验证')
          }
          return
        }
      }
      await api.sendRegisterCode(email.trim(), captcha)
      toast.success('验证码已发送，请查收邮箱')
      setCooldown(60)
    } catch (err) {
      const code = err instanceof ApiError ? err.code : ''
      toast.error(errMsg[code] || '发送失败，请稍后重试')
    } finally {
      setSending(false)
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (password !== confirm) {
      toast.error('两次输入的密码不一致')
      return
    }
    if (password.length < 8) {
      toast.error('密码至少 8 位')
      return
    }
    setLoading(true)
    try {
      const res = await api.register({
        email: email.trim(),
        code: code.trim(),
        password,
        confirm_password: confirm,
      })
      setSession(res.token, res.renew, res.user)
      toast.success('注册成功')
      await navigate({ to: '/' })
    } catch (err) {
      const code = err instanceof ApiError ? err.code : ''
      toast.error(errMsg[code] || '注册失败，请稍后重试')
    } finally {
      setLoading(false)
    }
  }

  const sendDisabled =
    !registerEnabled || sending || cooldown > 0 || (captchaEnabled && !captchaReady)

  return (
    <div className="flex min-h-svh items-center justify-center px-4 py-8 pt-[max(2rem,env(safe-area-inset-top))] pb-[max(2rem,env(safe-area-inset-bottom))]">
      <Card className="w-full max-w-md border-border/80 shadow-lg shadow-primary/5">
        <CardHeader className="space-y-4">
          <BrandMark size="lg" />
          <p className="text-sm text-muted-foreground">使用邮箱验证码创建商户账号</p>
        </CardHeader>
        <CardContent>
          {configLoaded && !registerEnabled ? (
            <div className="space-y-4">
              <p className="text-sm text-muted-foreground">注册已关闭，请联系管理员开通账号。</p>
              <Button asChild className="h-11 w-full">
                <Link to="/login">去登录</Link>
              </Button>
            </div>
          ) : (
            <form className="space-y-4" onSubmit={onSubmit}>
              <div className="space-y-2">
                <Label htmlFor="email">邮箱</Label>
                <Input
                  id="email"
                  type="email"
                  autoComplete="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  className="h-11"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="code">验证码</Label>
                <div className="flex gap-2">
                  <Input
                    id="code"
                    inputMode="numeric"
                    value={code}
                    onChange={(e) => setCode(e.target.value)}
                    required
                    className="h-11"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    className="h-11 shrink-0 px-3"
                    disabled={sendDisabled}
                    onClick={onSendCode}
                  >
                    {cooldown > 0
                      ? `${cooldown}s`
                      : sending
                        ? '发送中…'
                        : captchaEnabled && !captchaReady
                          ? '验证加载中…'
                          : '获取验证码'}
                  </Button>
                </div>
                {captchaBootError ? (
                  <p className="text-xs text-destructive">{captchaBootError}</p>
                ) : null}
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">密码</Label>
                <Input
                  id="password"
                  type="password"
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  minLength={8}
                  className="h-11"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="confirm">确认密码</Label>
                <Input
                  id="confirm"
                  type="password"
                  autoComplete="new-password"
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                  required
                  minLength={8}
                  className="h-11"
                />
              </div>
              <Button type="submit" className="h-11 w-full" disabled={loading || !registerEnabled}>
                {loading ? '注册中…' : '注册并登录'}
              </Button>
              <p className="text-center text-sm text-muted-foreground">
                已有账号？{' '}
                <Link to="/login" className="font-semibold text-primary underline-offset-4 hover:underline">
                  去登录
                </Link>
              </p>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
