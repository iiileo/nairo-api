import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { App, Button, Card, Form, Input, Typography, theme } from 'antd'
import { ApiError, api } from '@/lib/api'
import { getToken, setSession } from '@/lib/auth'

export const Route = createFileRoute('/login')({
  beforeLoad: () => {
    if (getToken()) throw redirect({ to: '/' })
  },
  component: LoginPage,
})

function LoginPage() {
  const navigate = useNavigate()
  const { message } = App.useApp()
  const { token } = theme.useToken()
  const [form] = Form.useForm()

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 24,
        background: token.colorBgLayout,
      }}
    >
      <Card style={{ width: 400, boxShadow: token.boxShadowTertiary }}>
        <div style={{ textAlign: 'center', marginBottom: 28 }}>
          <div
            style={{
              width: 40,
              height: 40,
              borderRadius: 8,
              background: token.colorPrimary,
              color: '#fff',
              display: 'inline-grid',
              placeItems: 'center',
              fontWeight: 700,
              fontSize: 18,
              marginBottom: 12,
            }}
          >
            N
          </div>
          <Typography.Title level={3} style={{ margin: 0 }}>
            Nairo Admin
          </Typography.Title>
          <Typography.Text type="secondary">平台管理后台</Typography.Text>
        </div>
        <Form
          form={form}
          layout="vertical"
          requiredMark={false}
          initialValues={{ username: 'admin', password: 'admin123' }}
          onFinish={async (values) => {
            try {
              const res = await api.login(values.username, values.password)
              setSession(res.token, res.admin)
              message.success('登录成功')
              await navigate({ to: '/' })
            } catch (err) {
              message.error(err instanceof ApiError ? '用户名或密码错误' : '登录失败')
            }
          }}
        >
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input size="large" autoComplete="username" placeholder="用户名" />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password size="large" autoComplete="current-password" placeholder="密码" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="primary" htmlType="submit" size="large" block>
              登录
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}
