import { Outlet, createRootRoute } from '@tanstack/react-router'
import { ConfirmProvider } from '@/components/ConfirmDialog'
import { Toaster } from '@/components/ui/sonner'
import '../styles.css'

export const Route = createRootRoute({
  component: () => (
    <ConfirmProvider>
      <Outlet />
      <Toaster richColors position="top-center" />
    </ConfirmProvider>
  ),
})
