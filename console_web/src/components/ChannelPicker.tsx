import { AlipayIcon, WechatIcon } from '@/components/ChannelIcons'
import { CHANNEL_BRAND, CHANNEL_OPTS } from '@/lib/channels'
import { cn } from '@/lib/utils'

type Props = {
  value: string
  onChange: (channel: string) => void
  /** 为空则全部可选 */
  enabledIds?: string[]
  className?: string
}

const icons = {
  wechat: WechatIcon,
  alipay: AlipayIcon,
} as const

export function ChannelPicker({ value, onChange, enabledIds, className }: Props) {
  return (
    <div className={cn('grid grid-cols-2 gap-2', className)} role="group" aria-label="支付渠道">
      {CHANNEL_OPTS.map((opt) => {
        const enabled = !enabledIds || enabledIds.includes(opt.id)
        const on = value === opt.id
        const meta = CHANNEL_BRAND[opt.id]
        const Icon = icons[opt.id]
        return (
          <button
            key={opt.id}
            type="button"
            disabled={!enabled}
            aria-pressed={on}
            onClick={() => onChange(opt.id)}
            className={cn(
              'flex h-12 items-center justify-center gap-2 rounded-xl border px-3 text-sm font-semibold transition-colors disabled:cursor-not-allowed disabled:opacity-40',
              on
                ? meta.active
                : 'border-border bg-card text-muted-foreground hover:bg-muted/60 hover:text-foreground',
            )}
          >
            <Icon className={cn('size-5', on ? meta.icon : 'opacity-70')} />
            <span>{opt.label}</span>
          </button>
        )
      })}
    </div>
  )
}
