import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

export type ConfirmOptions = {
  title?: string
  description: string
  confirmText?: string
  cancelText?: string
  /** 危险操作（删除、重置、解绑等）使用红色确认按钮 */
  destructive?: boolean
}

type ConfirmFn = (opts: ConfirmOptions) => Promise<boolean>

const ConfirmContext = createContext<ConfirmFn | null>(null)

/** 全局自定义确认框，替代 window.confirm。挂在根路由上，整个控制台共用一个。 */
export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<{ opts: ConfirmOptions; open: boolean } | null>(null)
  const resolverRef = useRef<((v: boolean) => void) | null>(null)

  const confirm = useCallback<ConfirmFn>(
    (opts) =>
      new Promise<boolean>((resolve) => {
        // 若上一个还没结束，先按取消处理
        resolverRef.current?.(false)
        resolverRef.current = resolve
        setState({ opts, open: true })
      }),
    [],
  )

  function settle(v: boolean) {
    resolverRef.current?.(v)
    resolverRef.current = null
    // 只关闭，保留内容，避免关闭动画期间文字闪动
    setState((cur) => (cur ? { ...cur, open: false } : cur))
  }

  const opts = state?.opts
  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      <Dialog open={!!state?.open} onOpenChange={(open) => !open && settle(false)}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>{opts?.title ?? '请确认'}</DialogTitle>
            <DialogDescription className="text-sm leading-relaxed">{opts?.description}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => settle(false)}>
              {opts?.cancelText ?? '取消'}
            </Button>
            <Button
              variant={opts?.destructive ? 'destructive' : 'default'}
              onClick={() => settle(true)}
            >
              {opts?.confirmText ?? '确认'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ConfirmContext.Provider>
  )
}

/** 返回 confirm(opts)：用户确认为 true，取消/关闭为 false。 */
export function useConfirm(): ConfirmFn {
  const fn = useContext(ConfirmContext)
  if (!fn) throw new Error('useConfirm 必须在 ConfirmProvider 内使用')
  return fn
}
