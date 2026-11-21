import { Link, useNavigate, useRouterState } from '@tanstack/react-router'
import {
  Bell,
  BookOpen,
  Code2,
  LayoutDashboard,
  ListOrdered,
  LogOut,
  Menu,
  MoreHorizontal,
  MonitorSmartphone,
  Settings2,
  Unlink,
  Wallet,
} from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { BrandMark } from '@/components/BrandMark'
import { api } from '@/lib/api'
import { getCachedUser } from '@/lib/auth'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'

type NavItem = {
  to: string
  label: string
  icon: typeof LayoutDashboard
  match?: (pathname: string) => boolean
}

type NavGroup = {
  title: string
  items: NavItem[]
}

const navGroups: NavGroup[] = [
  {
    title: '经营',
    items: [
      { to: '/', label: '总览', icon: LayoutDashboard },
      { to: '/accounts', label: '监控设备', icon: MonitorSmartphone },
      { to: '/orders', label: '订单', icon: ListOrdered },
      { to: '/orphans', label: '未匹配到账', icon: Unlink },
    ],
  },
  {
    title: '配置',
    items: [
      { to: '/settings', label: '金额微调', icon: Settings2 },
      { to: '/notifications', label: '通知', icon: Bell },
      { to: '/wallet', label: '钱包流水', icon: Wallet },
    ],
  },
  {
    title: '开发与帮助',
    items: [
      { to: '/developers', label: '开发者 API', icon: Code2 },
      { to: '/guide', label: '使用教程', icon: BookOpen },
    ],
  },
]

const flatNav = navGroups.flatMap((g) => g.items)

const mobilePrimary = [
  { to: '/', label: '总览', icon: LayoutDashboard },
  { to: '/accounts', label: '设备', icon: MonitorSmartphone },
  { to: '/orders', label: '订单', icon: ListOrdered },
] as const

function isActive(item: { to: string; match?: (pathname: string) => boolean }, pathname: string) {
  if (item.match) return item.match(pathname)
  if (item.to === '/') return pathname === '/' || pathname === ''
  return pathname === item.to || pathname.startsWith(`${item.to}/`)
}

function isPrimaryPath(pathname: string) {
  // 收款码子页仍算「设备」主路径，底部导航高亮设备
  if (pathname.startsWith('/accounts')) return true
  return mobilePrimary.some((item) => isActive(item, pathname))
}

function NavLinks({
  pathname,
  onNavigate,
  className,
}: {
  pathname: string
  onNavigate?: () => void
  className?: string
}) {
  return (
    <nav className={cn('flex flex-col gap-5', className)}>
      {navGroups.map((group) => (
        <div key={group.title} className="space-y-1.5">
          <div className="px-3 text-[11px] font-semibold tracking-[0.14em] text-muted-foreground uppercase">
            {group.title}
          </div>
          <div className="flex flex-col gap-0.5">
            {group.items.map((item) => {
              const active = isActive(item, pathname)
              const Icon = item.icon
              return (
                <Link
                  key={item.to}
                  to={item.to}
                  onClick={onNavigate}
                  className={cn(
                    'group flex min-h-11 items-center gap-2.5 rounded-xl px-3 py-2.5 text-sm font-semibold transition-all',
                    active
                      ? 'bg-primary text-primary-foreground shadow-md shadow-primary/20'
                      : 'text-sidebar-foreground hover:bg-muted/80',
                  )}
                >
                  <Icon
                    className={cn(
                      'size-4 shrink-0 transition-opacity',
                      active ? 'opacity-100' : 'opacity-70 group-hover:opacity-100',
                    )}
                  />
                  {item.label}
                </Link>
              )
            })}
          </div>
        </div>
      ))}
    </nav>
  )
}

function SideFooter({ onLogout }: { onLogout: () => void }) {
  const user = getCachedUser()
  return (
    <div className="mt-auto space-y-3 border-t border-sidebar-border pt-4">
      <div className="rounded-xl border border-sidebar-border bg-background/70 px-3 py-2.5">
        <div className="text-[11px] font-medium text-muted-foreground">当前账号</div>
        <div className="mt-0.5 truncate font-mono text-xs">
          {user?.phone || user?.email || user?.id}
        </div>
      </div>
      <Button variant="outline" className="min-h-11 w-full justify-start gap-2" onClick={onLogout}>
        <LogOut className="size-4" />
        退出登录
      </Button>
    </div>
  )
}

export function Shell({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const [open, setOpen] = useState(false)

  useEffect(() => {
    const tick = () => {
      api.heartbeat().catch(() => {})
    }
    tick()
    const id = window.setInterval(tick, 60_000)
    return () => window.clearInterval(id)
  }, [])

  function logout() {
    void api.logout().finally(() => {
      void navigate({ to: '/login' })
    })
  }

  const title = flatNav.find((n) => isActive(n, pathname))?.label || '控制台'
  const moreActive = !isPrimaryPath(pathname)

  return (
    <div className="min-h-svh md:grid md:grid-cols-[268px_minmax(0,1fr)]">
      <aside className="sticky top-0 hidden h-svh flex-col gap-6 border-r border-sidebar-border bg-sidebar/95 px-4 py-6 backdrop-blur md:flex">
        <div className="border-b border-sidebar-border px-1 pb-5">
          <BrandMark />
        </div>
        <div className="flex-1 overflow-y-auto pr-1">
          <NavLinks pathname={pathname} />
        </div>
        <SideFooter onLogout={logout} />
      </aside>

      <div className="flex min-w-0 flex-col">
        <header className="sticky top-0 z-40 flex items-center gap-3 border-b border-border bg-background/95 px-3 pb-2.5 pt-[max(0.625rem,env(safe-area-inset-top))] backdrop-blur md:hidden">
          <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
              <Button variant="outline" size="icon" className="size-11 shrink-0" aria-label="打开菜单">
                <Menu className="size-5" />
              </Button>
            </SheetTrigger>
            <SheetContent
              side="left"
              className="flex w-[min(100%,20rem)] flex-col overflow-y-auto p-4 pb-[max(1rem,env(safe-area-inset-bottom))]"
            >
              <SheetHeader className="px-1 text-left">
                <SheetTitle className="sr-only">导航菜单</SheetTitle>
                <BrandMark size="sm" />
              </SheetHeader>
              <Separator className="my-4" />
              <div className="flex-1 overflow-y-auto">
                <NavLinks pathname={pathname} onNavigate={() => setOpen(false)} />
              </div>
              <div className="mt-6">
                <SideFooter
                  onLogout={() => {
                    setOpen(false)
                    logout()
                  }}
                />
              </div>
            </SheetContent>
          </Sheet>
          <div className="min-w-0 flex-1">
            <div className="font-display text-base font-bold leading-none">Nairo</div>
            <div className="truncate text-xs text-muted-foreground">{title}</div>
          </div>
        </header>

        <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-4 pb-[calc(5.25rem+env(safe-area-inset-bottom))] sm:px-6 sm:py-8 md:pb-8">
          {children}
        </main>

        <nav
          className="fixed inset-x-0 bottom-0 z-40 border-t border-border bg-background/95 backdrop-blur md:hidden"
          style={{ paddingBottom: 'env(safe-area-inset-bottom)' }}
        >
          <div className="mx-auto grid max-w-6xl grid-cols-4 gap-1 px-2 py-1.5">
            {mobilePrimary.map((item) => {
              const active = isActive(item, pathname)
              const Icon = item.icon
              return (
                <Link
                  key={item.to}
                  to={item.to}
                  className={cn(
                    'flex min-h-12 flex-col items-center justify-center gap-0.5 rounded-xl px-1 text-[11px] font-semibold transition-colors',
                    active ? 'bg-primary/10 text-primary' : 'text-muted-foreground',
                  )}
                >
                  <Icon className="size-5" />
                  {item.label}
                </Link>
              )
            })}
            <button
              type="button"
              onClick={() => setOpen(true)}
              className={cn(
                'flex min-h-12 flex-col items-center justify-center gap-0.5 rounded-xl px-1 text-[11px] font-semibold transition-colors',
                moreActive ? 'bg-primary/10 text-primary' : 'text-muted-foreground',
              )}
            >
              <MoreHorizontal className="size-5" />
              更多
            </button>
          </div>
        </nav>
      </div>
    </div>
  )
}
