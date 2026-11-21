export function fenToYuan(cents: number) {
  return (cents / 100).toFixed(2)
}

export function yuanToFen(yuan: string | number) {
  const n = typeof yuan === 'number' ? yuan : Number(yuan)
  if (!Number.isFinite(n)) return 0
  return Math.round(n * 100)
}

export function statusLabel(status: string) {
  const map: Record<string, string> = {
    pending: '待支付',
    paid: '已支付',
    expired: '已过期',
    closed: '已关闭',
    open: '待处理',
    bound: '已绑定',
    ignored: '已忽略',
    active: '正常',
    disabled: '停用',
    auto: '自动',
    manual: '人工',
    wechat: '微信',
    alipay: '支付宝',
  }
  return map[status] || status
}
