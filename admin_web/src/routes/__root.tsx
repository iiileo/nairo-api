import { Outlet, createRootRoute } from '@tanstack/react-router'
import { App, ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import '../styles.css'

export const Route = createRootRoute({
  component: RootComponent,
})

function RootComponent() {
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        token: {
          borderRadius: 6,
          colorPrimary: '#2563eb',
          colorBgLayout: '#f1f5f9',
          controlHeight: 32,
        },
        components: {
          Layout: {
            headerBg: '#ffffff',
            headerHeight: 56,
            siderBg: '#ffffff',
            bodyBg: '#f5f5f5',
            triggerBg: '#ffffff',
            triggerColor: 'rgba(0,0,0,0.65)',
          },
          Menu: {
            itemBorderRadius: 6,
            itemMarginInline: 8,
            itemHeight: 40,
          },
          Table: {
            headerBg: '#fafafa',
          },
        },
      }}
    >
      <App>
        <Outlet />
      </App>
    </ConfigProvider>
  )
}
