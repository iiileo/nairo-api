import { cn } from '@/lib/utils'

export function BrandMark({
  size = 'md',
  showWordmark = true,
  className,
}: {
  size?: 'sm' | 'md' | 'lg'
  showWordmark?: boolean
  className?: string
}) {
  const box =
    size === 'lg' ? 'size-11 text-base' : size === 'sm' ? 'size-8 text-xs' : 'size-9 text-sm'
  const title = size === 'lg' ? 'text-2xl' : size === 'sm' ? 'text-base' : 'text-xl'

  return (
    <div className={cn('flex items-center gap-2.5', className)}>
      <div
        className={cn(
          'flex items-center justify-center rounded-xl bg-primary font-display font-bold tracking-tight text-primary-foreground shadow-sm shadow-primary/25',
          box,
        )}
        aria-hidden
      >
        NK
      </div>
      {showWordmark ? (
        <div className="min-w-0 leading-tight">
          <div className={cn('font-display font-bold tracking-tight', title)}>Nairo</div>
          <div className="text-xs text-muted-foreground">奈洛 · 商户控制台</div>
        </div>
      ) : null}
    </div>
  )
}
