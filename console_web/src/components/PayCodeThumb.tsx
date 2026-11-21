import { QrCode } from 'lucide-react'
import QRCode from 'qrcode'
import { useEffect, useState } from 'react'
import { cn } from '@/lib/utils'

export function PayCodeThumb({ content, large }: { content?: string | null; large?: boolean }) {
  const [src, setSrc] = useState('')
  const size = large ? 140 : 96
  useEffect(() => {
    let cancelled = false
    const text = content?.trim() || ''
    if (!text) {
      setSrc('')
      return
    }
    if (text.startsWith('http://') || text.startsWith('https://') || text.startsWith('data:')) {
      if (/\.(png|jpe?g|webp|gif)(\?|$)/i.test(text) || text.startsWith('data:image')) {
        setSrc(text)
        return
      }
    }
    void QRCode.toDataURL(text, { width: size, margin: 1 }).then((url) => {
      if (!cancelled) setSrc(url)
    })
    return () => {
      cancelled = true
    }
  }, [content, size])
  if (!src) {
    return (
      <div
        className={cn(
          'flex shrink-0 items-center justify-center rounded-md border bg-muted',
          large ? 'size-36' : 'size-16',
        )}
      >
        <QrCode className={cn('text-muted-foreground', large ? 'size-8' : 'size-5')} />
      </div>
    )
  }
  return (
    <img
      src={src}
      alt=""
      className={cn('shrink-0 rounded-md border object-cover', large ? 'size-36' : 'size-16')}
    />
  )
}
