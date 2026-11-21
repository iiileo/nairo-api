export function yuanFromCents(cents: number | null | undefined): string {
  const n = Number(cents ?? 0)
  return (n / 100).toFixed(2)
}

export function feeLabel(bps: number | null | undefined): string {
  const n = Number(bps ?? 0)
  return `${(n / 100).toFixed(2)}%`
}

export function shortTime(v: string | null | undefined): string {
  if (!v) return '—'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  return d.toLocaleString('zh-CN', { hour12: false })
}
