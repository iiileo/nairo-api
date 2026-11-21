export const CHANNEL_OPTS = [
  { id: 'wechat', label: '微信' },
  { id: 'alipay', label: '支付宝' },
] as const

/** 微信 / 支付宝品牌色，列表与选择器共用 */
export const CHANNEL_BRAND = {
  wechat: {
    accent: '#07C160',
    chip: 'border-[#07C160]/35 bg-[#07C160]/10 text-[#059669]',
    active: 'border-[#07C160] bg-[#07C160]/12 text-[#059669] ring-1 ring-[#07C160]/35',
    icon: 'text-[#07C160]',
    card: 'border-l-[#07C160]',
  },
  alipay: {
    accent: '#1677FF',
    chip: 'border-[#1677FF]/35 bg-[#1677FF]/10 text-[#1d4ed8]',
    active: 'border-[#1677FF] bg-[#1677FF]/12 text-[#1d4ed8] ring-1 ring-[#1677FF]/35',
    icon: 'text-[#1677FF]',
    card: 'border-l-[#1677FF]',
  },
} as const

export function channelBrand(channel: string) {
  return channel === 'alipay' ? CHANNEL_BRAND.alipay : CHANNEL_BRAND.wechat
}

export function accountChannels(acc: { channels?: string[]; channel: string }): string[] {
  if (acc.channels?.length) return acc.channels
  return acc.channel
    .split(',')
    .map((x) => x.trim())
    .filter(Boolean)
}
