import { Link, createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { useEffect, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { BrandMark } from '@/components/BrandMark'
import { ApiError, api } from '@/lib/api'
import { getToken, setSession } from '@/lib/auth'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export const Route = createFileRoute('/login')({
  beforeLoad: () => {
    if (getToken()) throw redirect({ to: '/' })
  },
  component: LoginPage,
})

function LoginPage() {
  const navigate = useNavigate()
  const [login, setLogin] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [registerEnabled, setRegisterEnabled] = useState(true)

  useEffect(() => {
    void api
      .geetest()
      .then((res) => setRegisterEnabled(res.register_enabled !== false))
      .catch(() => setRegisterEnabled(true))
  }, [])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      const res = await api.login(login.trim(), password)
      setSession(res.token, res.renew, res.user)
      toast.success('登录成功')
      await navigate({ to: '/' })
    } catch (err) {
      toast.error(err instanceof ApiError ? '账号或密码错误' : '登录失败，请稍后重试')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-svh items-center justify-center px-4 py-8 pt-[max(2rem,env(safe-area-inset-top))] pb-[max(2rem,env(safe-area-inset-bottom))]">
      <Card className="w-full max-w-md border-border/80 shadow-lg shadow-primary/5">
        <CardHeader className="space-y-4">
          <BrandMark size="lg" />
          <p className="text-sm text-muted-foreground">邮箱或手机号登录商户控制台</p>
        </CardHeader>
        <CardContent>
          <form className="space-y-4" onSubmit={onSubmit}>
            <div className="space-y-2">
              <Label htmlFor="login">手机号 / 邮箱</Label>
              <Input
                id="login"
                autoComplete="username"
                value={login}
                onChange={(e) => setLogin(e.target.value)}
                required
                className="h-11"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">密码</Label>
              <Input
                id="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                className="h-11"
              />
            </div>
            <Button type="submit" className="h-11 w-full" disabled={loading}>
              {loading ? '登录中…' : '登录'}
            </Button>
            {registerEnabled ? (
              <p className="text-center text-sm text-muted-foreground">
                还没有账号？{' '}
                <Link to="/register" className="font-semibold text-primary underline-offset-4 hover:underline">
                  邮箱注册
                </Link>
              </p>
            ) : null}
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
