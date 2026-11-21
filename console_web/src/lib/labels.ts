const orderStatus: Record<string, string> = {
  pending: '待支付',
  paid: '已支付',
  expired: '已过期',
  closed: '已关闭',
}

const settleType: Record<string, string> = {
  auto: '自动入账',
  manual: '人工补单',
}

const orphanStatus: Record<string, string> = {
  open: '待处理',
  bound: '已绑定',
  ignored: '已忽略',
}

const channel: Record<string, string> = {
  wechat: '微信',
  alipay: '支付宝',
}

const accountStatus: Record<string, string> = {
  active: '启用',
  disabled: '停用',
}

const codeType: Record<string, string> = {
  float: '任意金额',
  fixed: '固额',
}

const ledgerType: Record<string, string> = {
  recharge: '充值',
  fee_deduct: '手续费扣除',
  adjust: '调整',
}

const adjustDirection: Record<string, string> = {
  inc: '递增',
  dec: '递减',
}

export const labels = {
  orderStatus: (v?: string | null) => (v && orderStatus[v]) || v || '—',
  settleType: (v?: string | null) => (v && settleType[v]) || v || '—',
  orphanStatus: (v?: string | null) => (v && orphanStatus[v]) || v || '—',
  channel: (v?: string | null) => {
    if (!v) return '—'
    if (channel[v]) return channel[v]
    return v
      .split(',')
      .map((p) => channel[p.trim()] || p.trim())
      .filter(Boolean)
      .join(' + ') || v
  },
  channels: (list?: string[] | null) => {
    if (!list?.length) return '—'
    return list.map((p) => channel[p] || p).join(' + ')
  },
  accountStatus: (v?: string | null) => (v && accountStatus[v]) || v || '—',
  codeType: (v?: string | null) => (v && codeType[v]) || v || '—',
  ledgerType: (v?: string | null) => (v && ledgerType[v]) || v || '—',
  adjustDirection: (v?: string | null) => (v && adjustDirection[v]) || v || '—',
}
