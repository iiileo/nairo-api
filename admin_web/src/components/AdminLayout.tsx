import {
  ListOrdered,
  LogOut,
  Menu as MenuIcon,
  Settings2,
  Unlink,
  User,
  Users,
} from 'lucide-react'
import { Avatar, Breadcrumb, Button, Dropdown, Layout, Menu, Space, Typography, theme } from 'antd'
import { useNavigate, useRouterState } from '@tanstack/react-router'
import { useMemo, useState, type ReactNode } from 'react'
import { clearSession, getCachedAdmin } from '@/lib/auth'

const { Header, Sider, Content } = Layout

const iconProps = { size: 16, strokeWidth: 2 }

const menuItems = [
  { key: '/', icon: <Users {...iconProps} />, label: '商户用户' },
  { key: '/orders', icon: <ListOrdered {...iconProps} />, label: '订单管理' },
  { key: '/orphans', icon: <Unlink {...iconProps} />, label: '未匹配到账' },
  { key: '/settings', icon: <Settings2 {...iconProps} />, label: '系统设置' },
]

const titles: Record<string, string> = {
  '/': '商户用户',
  '/orders': '订单管理',
  '/orphans': '未匹配到账',
  '/settings': '系统设置',
}

export function AdminLayout({
  children,
  extra,
}: {
  children: ReactNode
  extra?: ReactNode
}) {
  const navigate = useNavigate()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const { token } = theme.useToken()
  const admin = getCachedAdmin()
  const [collapsed, setCollapsed] = useState(false)

  const selected =
    menuItems.find((i) => (i.key === '/' ? pathname === '/' : pathname.startsWith(i.key)))?.key ||
    '/'
  const pageTitle = titles[selected] || '管理后台'

  const breadcrumb = useMemo(
    () => [{ title: '管理后台' }, { title: pageTitle }],
    [pageTitle],
  )

  return (
    <Layout style={{ minHeight: '100vh' }} hasSider>
      <Sider
        className="admin-sider"
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        width={220}
        theme="light"
        style={{
          background: '#fff',
          borderInlineEnd: `1px solid ${token.colorBorderSecondary}`,
          overflow: 'auto',
          height: '100vh',
          position: 'sticky',
          top: 0,
          insetInlineStart: 0,
        }}
      >
        <div
          style={{
            height: 56,
            paddingInline: collapsed ? 0 : 20,
            display: 'flex',
            alignItems: 'center',
            justifyContent: collapsed ? 'center' : 'flex-start',
            gap: 10,
            borderBottom: `1px solid ${token.colorBorderSecondary}`,
            flexShrink: 0,
          }}
        >
          <div
            style={{
              width: 28,
              height: 28,
              borderRadius: 6,
              background: token.colorPrimary,
              color: '#fff',
              display: 'grid',
              placeItems: 'center',
              fontWeight: 700,
              fontSize: 14,
              flexShrink: 0,
            }}
          >
            N
          </div>
          {!collapsed ? (
            <Typography.Text strong style={{ fontSize: 15 }}>
              Nairo Admin
            </Typography.Text>
          ) : null}
        </div>
        <Menu
          theme="light"
          mode="inline"
          selectedKeys={[selected]}
          items={menuItems}
          style={{ flex: 1, paddingTop: 8, borderInlineEnd: 'none' }}
          onClick={({ key }) => {
            void navigate({ to: key })
          }}
        />
      </Sider>

      <Layout style={{ minWidth: 0 }}>
        <Header
          style={{
            paddingInline: 20,
            background: '#fff',
            borderBottom: `1px solid ${token.colorBorderSecondary}`,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 16,
            position: 'sticky',
            top: 0,
            zIndex: 10,
          }}
        >
          <Space size={12}>
            <Button
              type="text"
              icon={<MenuIcon {...iconProps} />}
              onClick={() => setCollapsed((v) => !v)}
            />
            <Breadcrumb items={breadcrumb} />
          </Space>
          <Dropdown
            menu={{
              items: [
                {
                  key: 'logout',
                  icon: <LogOut {...iconProps} />,
                  label: '退出登录',
                  onClick: () => {
                    clearSession()
                    void navigate({ to: '/login' })
                  },
                },
              ],
            }}
          >
            <Space style={{ cursor: 'pointer' }}>
              <Avatar
                size={28}
                icon={<User size={14} strokeWidth={2} />}
                style={{ background: token.colorPrimary }}
              />
              <Typography.Text>{admin?.username || 'admin'}</Typography.Text>
            </Space>
          </Dropdown>
        </Header>

        <Content style={{ padding: 20 }}>
          <div
            style={{
              marginBottom: 16,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 12,
              flexWrap: 'wrap',
            }}
          >
            <Typography.Title level={4} style={{ margin: 0 }}>
              {pageTitle}
            </Typography.Title>
            {extra ? <Space wrap>{extra}</Space> : null}
          </div>
          {children}
        </Content>
      </Layout>
    </Layout>
  )
}
