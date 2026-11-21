import { createFileRoute, redirect } from '@tanstack/react-router'
import {
  CreditCard,
  Mail,
  MinusCircle,
  Plus,
  RefreshCw,
  ScanLine,
  ShieldCheck,
  Store,
  UsersRound,
} from 'lucide-react'
import {
  App,
  Button,
  Card,
  Col,
  Form,
  Input,
  InputNumber,
  Layout,
  Menu,
  Row,
  Select,
  Space,
  Switch,
  Tabs,
  Typography,
} from 'antd'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { AdminLayout } from '@/components/AdminLayout'
import {
  ApiError,
  api,
  type AmountPattern,
  type AppMatchRules,
  type RuntimeConfigPatch,
  type UserItem,
} from '@/lib/api'
import { getToken } from '@/lib/auth'

const icon = { size: 14, strokeWidth: 2 }
const menuIcon = { size: 15, strokeWidth: 2 }

type SettingsTab =
  | 'smtp'
  | 'pay'
  | 'console'
  | 'geetest'
  | 'pay_user'
  | 'match_rules'

const TAB_META: Record<
  SettingsTab,
  { label: string; title: string; desc: string; icon: ReactNode }
> = {
  smtp: {
    label: '邮件 SMTP',
    title: '邮件 SMTP',
    desc: '注册验证码等邮件发送；保存后立即热加载。',
    icon: <Mail {...menuIcon} />,
  },
  pay: {
    label: '支付参数',
    title: '支付参数',
    desc: '公网收银台地址与金额锁 TTL。',
    icon: <CreditCard {...menuIcon} />,
  },
  console: {
    label: '商户 Console',
    title: '商户 Console',
    desc: '默认费率、验证码有效期与是否开放自助注册。',
    icon: <UsersRound {...menuIcon} />,
  },
  geetest: {
    label: '人机验证',
    title: '极验 GT4',
    desc: '注册发码人机验证；ID 与 Key 同时配置后启用。',
    icon: <ShieldCheck {...menuIcon} />,
  },
  pay_user: {
    label: '收款商户',
    title: '平台收款商户',
    desc: '用于公开试付与平台代收场景的商户账户。',
    icon: <Store {...menuIcon} />,
  },
  match_rules: {
    label: '匹配规则',
    title: 'App 收款通知匹配规则',
    desc: 'App 会定期拉取；保存后绑定设备的 App 在下次刷新生效。',
    icon: <ScanLine {...menuIcon} />,
  },
}

export const Route = createFileRoute('/settings')({
  beforeLoad: () => {
    if (!getToken()) throw redirect({ to: '/login' })
  },
  component: SystemSettingsPage,
})

function SystemSettingsPage() {
  const { message } = App.useApp()
  const [tab, setTab] = useState<SettingsTab>('smtp')
  const [payForm] = Form.useForm()
  const [rulesForm] = Form.useForm()
  const [runtimeForm] = Form.useForm()
  const [loading, setLoading] = useState(false)
  const [rulesLoading, setRulesLoading] = useState(false)
  const [runtimeLoading, setRuntimeLoading] = useState(false)
  const [savingPay, setSavingPay] = useState(false)
  const [savingRules, setSavingRules] = useState(false)
  const [savingRuntime, setSavingRuntime] = useState(false)
  const [hint, setHint] = useState('')
  const [rulesHint, setRulesHint] = useState('')
  const [runtimeHint, setRuntimeHint] = useState('')
  const [runtimeSource, setRuntimeSource] = useState('')
  const [passwordSet, setPasswordSet] = useState(false)
  const [geetestKeySet, setGeetestKeySet] = useState(false)
  const [ok, setOk] = useState(false)
  const [rulesMeta, setRulesMeta] = useState('')
  const [users, setUsers] = useState<UserItem[]>([])

  async function loadRuntime() {
    setRuntimeLoading(true)
    try {
      const res = await api.runtimeConfig()
      const c = res.config
      setRuntimeHint(res.hint)
      setRuntimeSource(c.source)
      setPasswordSet(c.smtp.password_set)
      setGeetestKeySet(!!c.geetest?.key_set)
      runtimeForm.setFieldsValue({
        smtp_host: c.smtp.host,
        smtp_port: c.smtp.port || 587,
        smtp_user: c.smtp.user,
        smtp_from: c.smtp.from,
        smtp_dev_print: c.smtp.dev_print,
        smtp_password: '',
        pay_public_base_url: c.pay.public_base_url,
        pay_lock_ttl_seconds: c.pay.lock_ttl_seconds,
        console_default_fee_rate_bps: c.console.default_fee_rate_bps,
        console_code_ttl_seconds: c.console.code_ttl_seconds,
        console_register_enabled: c.console.register_enabled !== false,
        geetest_captcha_id: c.geetest?.captcha_id || '',
        geetest_captcha_key: '',
      })
    } catch {
      message.error('运行时配置加载失败')
    } finally {
      setRuntimeLoading(false)
    }
  }

  async function loadPay() {
    setLoading(true)
    try {
      const [settings, userRes] = await Promise.all([api.platformSettings(), api.users(200, 0)])
      setUsers(userRes.items)
      payForm.setFieldsValue({ user_id: settings.pay_user_id || undefined })
      setHint(settings.hint)
      setOk(settings.pay_user_ok)
    } catch {
      message.error('加载失败')
    } finally {
      setLoading(false)
    }
  }

  async function loadRules() {
    setRulesLoading(true)
    try {
      const res = await api.matchRules()
      const r = res.rules
      setRulesHint(res.hint)
      setRulesMeta(`v${r.version}${r.updated_at ? ` · ${r.updated_at}` : ''}`)
      const packages = Object.entries(r.watch_packages || {}).map(([channel, package_name]) => ({
        channel,
        package_name,
      }))
      rulesForm.setFieldsValue({
        version: r.version,
        receive_keywords: r.receive_keywords || [],
        outbound_keywords: r.outbound_keywords || [],
        ignore_keywords: r.ignore_keywords || [],
        sanitize_patterns: r.sanitize_patterns || [],
        amount_patterns: (r.amount_patterns || []).map((p) => ({
          name: p.name,
          regex: p.regex,
          group: p.group || 1,
        })),
        watch_packages: packages.length
          ? packages
          : [
              { channel: 'wechat', package_name: 'com.tencent.mm' },
              { channel: 'alipay', package_name: 'com.eg.android.AlipayGphone' },
            ],
      })
    } catch {
      message.error('匹配规则加载失败')
    } finally {
      setRulesLoading(false)
    }
  }

  useEffect(() => {
    void loadRuntime()
    void loadPay()
    void loadRules()
  }, [])

  const userOptions = users.map((u) => ({
    value: u.id,
    label: [u.email, u.phone, u.id].filter(Boolean).join(' · '),
  }))

  const menuItems = useMemo(
    () =>
      (Object.keys(TAB_META) as SettingsTab[]).map((key) => ({
        key,
        icon: TAB_META[key].icon,
        label: TAB_META[key].label,
      })),
    [],
  )

  async function saveRuntimePatch(patch: RuntimeConfigPatch) {
    setSavingRuntime(true)
    try {
      await api.setRuntimeConfig(patch)
      message.success('已保存并热加载')
      await loadRuntime()
    } catch (err) {
      message.error(err instanceof ApiError ? err.code : '保存失败')
    } finally {
      setSavingRuntime(false)
    }
  }

  const meta = TAB_META[tab]
  const isRuntimeTab = tab === 'smtp' || tab === 'pay' || tab === 'console' || tab === 'geetest'

  return (
    <AdminLayout
      extra={
        <Button
          icon={<RefreshCw {...icon} />}
          onClick={() => {
            void loadRuntime()
            void loadPay()
            void loadRules()
          }}
        >
          全部刷新
        </Button>
      }
    >
      <Layout
        className="settings-split"
        style={{
          background: '#fff',
          borderRadius: 12,
          border: '1px solid #f0f0f0',
          overflow: 'hidden',
          minHeight: 560,
        }}
      >
        <Layout.Sider
          width={200}
          theme="light"
          style={{
            background: '#fafafa',
            borderInlineEnd: '1px solid #f0f0f0',
          }}
        >
          <Menu
            mode="inline"
            selectedKeys={[tab]}
            items={menuItems}
            style={{ borderInlineEnd: 'none', background: 'transparent', paddingTop: 8 }}
            onClick={({ key }) => setTab(key as SettingsTab)}
          />
        </Layout.Sider>

        <Layout.Content className="settings-panel" style={{ padding: 20, background: '#fff', minWidth: 0 }}>
          <div style={{ marginBottom: 16 }}>
            <Typography.Title level={5} style={{ margin: 0 }}>
              {meta.title}
            </Typography.Title>
            <Typography.Paragraph type="secondary" style={{ margin: '6px 0 0' }}>
              {meta.desc}
              {isRuntimeTab ? (
                <>
                  {' '}
                  来源：{runtimeSource === 'db' ? '数据库' : '环境变量'}
                  {runtimeHint ? ` · ${runtimeHint}` : ''}
                </>
              ) : null}
            </Typography.Paragraph>
          </div>

          {tab === 'smtp' ? (
            <Card
              size="small"
              loading={runtimeLoading}
              variant="borderless"
              styles={{ body: { padding: 0 }, root: { boxShadow: 'none' } }}
            >
              <Form
                form={runtimeForm}
                layout="vertical"
                onFinish={async (values) => {
                  const password = String(values.smtp_password || '')
                  await saveRuntimePatch({
                    smtp: {
                      host: values.smtp_host,
                      port: values.smtp_port,
                      user: values.smtp_user,
                      from: values.smtp_from,
                      dev_print: values.smtp_dev_print,
                      ...(password ? { password } : {}),
                    },
                  })
                }}
              >
                <Row gutter={16}>
                  <Col xs={24} md={10}>
                    <Form.Item name="smtp_host" label="Host">
                      <Input placeholder="smtp.example.com" />
                    </Form.Item>
                  </Col>
                  <Col xs={12} md={4}>
                    <Form.Item name="smtp_port" label="Port">
                      <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                  <Col xs={12} md={10}>
                    <Form.Item name="smtp_from" label="From">
                      <Input placeholder="noreply@example.com" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={8}>
                    <Form.Item name="smtp_user" label="User">
                      <Input autoComplete="off" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={8}>
                    <Form.Item
                      name="smtp_password"
                      label="Password"
                      extra={passwordSet ? '已配置；留空则保持不变' : '尚未配置'}
                    >
                      <Input.Password placeholder="留空不修改" autoComplete="new-password" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={8}>
                    <Form.Item
                      name="smtp_dev_print"
                      label="DevPrint（无 Host 时打日志）"
                      valuePropName="checked"
                    >
                      <Switch />
                    </Form.Item>
                  </Col>
                </Row>
                <RuntimeActions
                  saving={savingRuntime}
                  onReload={async () => {
                    try {
                      await api.reloadRuntimeConfig()
                      message.success('已从数据库重新加载')
                      await loadRuntime()
                    } catch (err) {
                      message.error(err instanceof ApiError ? err.code : '重载失败')
                    }
                  }}
                />
              </Form>
            </Card>
          ) : null}

          {tab === 'pay' ? (
            <Card
              size="small"
              loading={runtimeLoading}
              variant="borderless"
              styles={{ body: { padding: 0 }, root: { boxShadow: 'none' } }}
            >
              <Form
                form={runtimeForm}
                layout="vertical"
                onFinish={async (values) => {
                  await saveRuntimePatch({
                    pay: {
                      public_base_url: values.pay_public_base_url,
                      lock_ttl_seconds: values.pay_lock_ttl_seconds,
                    },
                  })
                }}
              >
                <Row gutter={16}>
                  <Col xs={24} md={16}>
                    <Form.Item
                      name="pay_public_base_url"
                      label="公网 Base URL"
                      extra="用于收银台 / 回调拼链接，如 https://pay.example.com"
                    >
                      <Input placeholder="https://pay.example.com" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={8}>
                    <Form.Item name="pay_lock_ttl_seconds" label="金额锁 TTL（秒）">
                      <InputNumber min={30} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                </Row>
                <RuntimeActions
                  saving={savingRuntime}
                  onReload={async () => {
                    try {
                      await api.reloadRuntimeConfig()
                      message.success('已从数据库重新加载')
                      await loadRuntime()
                    } catch (err) {
                      message.error(err instanceof ApiError ? err.code : '重载失败')
                    }
                  }}
                />
              </Form>
            </Card>
          ) : null}

          {tab === 'console' ? (
            <Card
              size="small"
              loading={runtimeLoading}
              variant="borderless"
              styles={{ body: { padding: 0 }, root: { boxShadow: 'none' } }}
            >
              <Form
                form={runtimeForm}
                layout="vertical"
                onFinish={async (values) => {
                  await saveRuntimePatch({
                    console: {
                      default_fee_rate_bps: values.console_default_fee_rate_bps,
                      code_ttl_seconds: values.console_code_ttl_seconds,
                      register_enabled: !!values.console_register_enabled,
                    },
                  })
                }}
              >
                <Row gutter={16}>
                  <Col xs={24} md={8}>
                    <Form.Item
                      name="console_default_fee_rate_bps"
                      label="默认费率（bps）"
                      extra="100 = 1%"
                    >
                      <InputNumber min={0} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={8}>
                    <Form.Item name="console_code_ttl_seconds" label="验证码有效期（秒）">
                      <InputNumber min={60} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={8}>
                    <Form.Item
                      name="console_register_enabled"
                      label="开放商户注册"
                      valuePropName="checked"
                      extra="关闭后无法自助发码/注册"
                    >
                      <Switch />
                    </Form.Item>
                  </Col>
                </Row>
                <RuntimeActions
                  saving={savingRuntime}
                  onReload={async () => {
                    try {
                      await api.reloadRuntimeConfig()
                      message.success('已从数据库重新加载')
                      await loadRuntime()
                    } catch (err) {
                      message.error(err instanceof ApiError ? err.code : '重载失败')
                    }
                  }}
                />
              </Form>
            </Card>
          ) : null}

          {tab === 'geetest' ? (
            <Card
              size="small"
              loading={runtimeLoading}
              variant="borderless"
              styles={{ body: { padding: 0 }, root: { boxShadow: 'none' } }}
            >
              <Form
                form={runtimeForm}
                layout="vertical"
                onFinish={async (values) => {
                  const geetestKey = String(values.geetest_captcha_key || '')
                  await saveRuntimePatch({
                    geetest: {
                      captcha_id: values.geetest_captcha_id,
                      ...(geetestKey ? { captcha_key: geetestKey } : {}),
                    },
                  })
                }}
              >
                <Row gutter={16}>
                  <Col xs={24} md={12}>
                    <Form.Item
                      name="geetest_captcha_id"
                      label="Captcha ID"
                      extra="前端仅暴露 ID；Key 仅服务端二次校验使用"
                    >
                      <Input placeholder="captcha_id" autoComplete="off" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item
                      name="geetest_captcha_key"
                      label="Captcha Key"
                      extra={geetestKeySet ? '已配置；留空则保持不变' : '尚未配置'}
                    >
                      <Input.Password placeholder="留空不修改" autoComplete="new-password" />
                    </Form.Item>
                  </Col>
                </Row>
                <RuntimeActions
                  saving={savingRuntime}
                  onReload={async () => {
                    try {
                      await api.reloadRuntimeConfig()
                      message.success('已从数据库重新加载')
                      await loadRuntime()
                    } catch (err) {
                      message.error(err instanceof ApiError ? err.code : '重载失败')
                    }
                  }}
                />
              </Form>
            </Card>
          ) : null}

          {tab === 'pay_user' ? (
            <Card
              size="small"
              loading={loading}
              variant="borderless"
              styles={{ body: { padding: 0 }, root: { boxShadow: 'none' } }}
            >
              <Typography.Paragraph style={{ marginBottom: 16 }}>
                状态：
                {ok ? (
                  <Typography.Text type="success">已配置且可用</Typography.Text>
                ) : (
                  <Typography.Text type="warning">未配置或商户不可用</Typography.Text>
                )}
                {hint ? (
                  <Typography.Text type="secondary"> · {hint}</Typography.Text>
                ) : null}
              </Typography.Paragraph>
              <Form
                form={payForm}
                layout="vertical"
                onFinish={async (values) => {
                  setSavingPay(true)
                  try {
                    await api.setPlatformPayUser(String(values.user_id || '').trim())
                    message.success('已保存')
                    await loadPay()
                  } catch (err) {
                    message.error(err instanceof ApiError ? err.code : '保存失败')
                  } finally {
                    setSavingPay(false)
                  }
                }}
              >
                <Row gutter={16}>
                  <Col xs={24} md={16} lg={12}>
                    <Form.Item
                      name="user_id"
                      label="收款商户"
                      extra="从商户列表选择；清空并保存可取消配置"
                    >
                      <Select
                        allowClear
                        showSearch
                        optionFilterProp="label"
                        placeholder="选择商户用户"
                        options={userOptions}
                      />
                    </Form.Item>
                  </Col>
                </Row>
                <Space>
                  <Button type="primary" htmlType="submit" loading={savingPay}>
                    保存
                  </Button>
                  <Button onClick={() => void loadPay()}>刷新</Button>
                </Space>
              </Form>
            </Card>
          ) : null}

          {tab === 'match_rules' ? (
            <Card
              size="small"
              loading={rulesLoading}
              variant="borderless"
              styles={{ body: { padding: 0 }, root: { boxShadow: 'none' } }}
              extra={<Typography.Text type="secondary">{rulesMeta}</Typography.Text>}
            >
              {rulesHint ? (
                <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
                  {rulesHint}
                </Typography.Paragraph>
              ) : null}
              <Form
                form={rulesForm}
                layout="vertical"
                onFinish={async (values) => {
                  setSavingRules(true)
                  try {
                    const amount_patterns = (values.amount_patterns || []) as AmountPattern[]
                    if (!amount_patterns.length) {
                      message.error('至少保留一条金额正则')
                      return
                    }
                    for (const p of amount_patterns) {
                      if (!String(p?.regex || '').trim()) {
                        message.error('金额正则不能为空')
                        return
                      }
                    }
                    const watch_packages: Record<string, string> = {}
                    for (const row of values.watch_packages || []) {
                      const ch = String(row?.channel || '').trim()
                      const pkg = String(row?.package_name || '').trim()
                      if (!ch || !pkg) continue
                      watch_packages[ch] = pkg
                    }
                    if (!Object.keys(watch_packages).length) {
                      message.error('至少配置一个监听包名')
                      return
                    }
                    const body: AppMatchRules = {
                      version: Number(values.version) || 1,
                      receive_keywords: values.receive_keywords || [],
                      outbound_keywords: values.outbound_keywords || [],
                      ignore_keywords: values.ignore_keywords || [],
                      sanitize_patterns: values.sanitize_patterns || [],
                      amount_patterns: amount_patterns.map((p) => ({
                        name: String(p.name || '').trim() || 'pattern',
                        regex: String(p.regex || '').trim(),
                        group: Number(p.group) > 0 ? Number(p.group) : 1,
                      })),
                      watch_packages,
                    }
                    await api.setMatchRules(body)
                    message.success('匹配规则已保存')
                    await loadRules()
                  } catch (err) {
                    message.error(err instanceof ApiError ? err.code : '保存失败')
                  } finally {
                    setSavingRules(false)
                  }
                }}
              >
                <Form.Item name="version" hidden>
                  <Input />
                </Form.Item>

                <Tabs
                  items={[
                    {
                      key: 'keywords',
                      label: '关键词',
                      children: (
                        <Row gutter={16}>
                          <Col xs={24} lg={8}>
                            <Form.Item
                              name="receive_keywords"
                              label="收款正向关键词"
                              extra="命中任一才视为可能的收款通知"
                            >
                              <Select
                                mode="tags"
                                tokenSeparators={[',', '，', '\n']}
                                placeholder="回车添加"
                              />
                            </Form.Item>
                          </Col>
                          <Col xs={24} lg={8}>
                            <Form.Item
                              name="ignore_keywords"
                              label="忽略关键词"
                              extra="提现、结算等到自己账户"
                            >
                              <Select
                                mode="tags"
                                tokenSeparators={[',', '，', '\n']}
                                placeholder="回车添加"
                              />
                            </Form.Item>
                          </Col>
                          <Col xs={24} lg={8}>
                            <Form.Item name="outbound_keywords" label="转出/支付噪声">
                              <Select
                                mode="tags"
                                tokenSeparators={[',', '，', '\n']}
                                placeholder="回车添加"
                              />
                            </Form.Item>
                          </Col>
                        </Row>
                      ),
                    },
                    {
                      key: 'amount',
                      label: '清洗与金额',
                      children: (
                        <>
                          <Form.Item
                            name="sanitize_patterns"
                            label="金额解析前清洗正则"
                            extra="例如去掉微信汇总角标 [2条]"
                          >
                            <Select
                              mode="tags"
                              tokenSeparators={['\n']}
                              placeholder="输入正则后回车"
                              optionLabelProp="value"
                            />
                          </Form.Item>
                          <Typography.Text strong style={{ display: 'block', marginBottom: 8 }}>
                            金额正则（按顺序匹配）
                          </Typography.Text>
                          <Form.List name="amount_patterns">
                            {(fields, { add, remove }) => (
                              <Space orientation="vertical" size={8} style={{ width: '100%' }}>
                                {fields.map((field) => (
                                  <Row key={field.key} gutter={8} align="top">
                                    <Col xs={24} sm={5}>
                                      <Form.Item
                                        {...field}
                                        name={[field.name, 'name']}
                                        rules={[{ required: true, message: '名称' }]}
                                        style={{ marginBottom: 8 }}
                                      >
                                        <Input placeholder="名称 yuan" />
                                      </Form.Item>
                                    </Col>
                                    <Col xs={24} sm={14}>
                                      <Form.Item
                                        {...field}
                                        name={[field.name, 'regex']}
                                        rules={[{ required: true, message: '正则必填' }]}
                                        style={{ marginBottom: 8 }}
                                      >
                                        <Input
                                          placeholder="正则"
                                          style={{ fontFamily: 'monospace' }}
                                        />
                                      </Form.Item>
                                    </Col>
                                    <Col xs={12} sm={3}>
                                      <Form.Item
                                        {...field}
                                        name={[field.name, 'group']}
                                        initialValue={1}
                                        style={{ marginBottom: 8 }}
                                      >
                                        <InputNumber
                                          min={1}
                                          style={{ width: '100%' }}
                                          placeholder="组"
                                        />
                                      </Form.Item>
                                    </Col>
                                    <Col xs={12} sm={2}>
                                      <Button
                                        type="text"
                                        danger
                                        icon={<MinusCircle {...icon} />}
                                        onClick={() => remove(field.name)}
                                        disabled={fields.length <= 1}
                                      />
                                    </Col>
                                  </Row>
                                ))}
                                <Button
                                  type="dashed"
                                  icon={<Plus {...icon} />}
                                  onClick={() => add({ name: '', regex: '', group: 1 })}
                                  block
                                >
                                  添加金额规则
                                </Button>
                              </Space>
                            )}
                          </Form.List>
                        </>
                      ),
                    },
                    {
                      key: 'packages',
                      label: '监听包名',
                      children: (
                        <Form.List name="watch_packages">
                          {(fields, { add, remove }) => (
                            <Space orientation="vertical" size={8} style={{ width: '100%' }}>
                              {fields.map((field) => (
                                <Row key={field.key} gutter={8}>
                                  <Col xs={24} sm={8}>
                                    <Form.Item
                                      {...field}
                                      name={[field.name, 'channel']}
                                      rules={[{ required: true, message: '渠道' }]}
                                      style={{ marginBottom: 8 }}
                                    >
                                      <Input placeholder="wechat / alipay" />
                                    </Form.Item>
                                  </Col>
                                  <Col xs={20} sm={14}>
                                    <Form.Item
                                      {...field}
                                      name={[field.name, 'package_name']}
                                      rules={[{ required: true, message: '包名' }]}
                                      style={{ marginBottom: 8 }}
                                    >
                                      <Input
                                        placeholder="com.tencent.mm"
                                        style={{ fontFamily: 'monospace' }}
                                      />
                                    </Form.Item>
                                  </Col>
                                  <Col xs={4} sm={2}>
                                    <Button
                                      type="text"
                                      danger
                                      icon={<MinusCircle {...icon} />}
                                      onClick={() => remove(field.name)}
                                      disabled={fields.length <= 1}
                                    />
                                  </Col>
                                </Row>
                              ))}
                              <Button
                                type="dashed"
                                icon={<Plus {...icon} />}
                                onClick={() => add({ channel: '', package_name: '' })}
                                block
                              >
                                添加包名
                              </Button>
                            </Space>
                          )}
                        </Form.List>
                      ),
                    },
                  ]}
                />

                <Space wrap style={{ marginTop: 8 }}>
                  <Button type="primary" htmlType="submit" loading={savingRules}>
                    保存规则
                  </Button>
                  <Button onClick={() => void loadRules()}>刷新</Button>
                  <Button
                    danger
                    onClick={async () => {
                      try {
                        await api.resetMatchRules()
                        message.success('已恢复默认规则')
                        await loadRules()
                      } catch (err) {
                        message.error(err instanceof ApiError ? err.code : '重置失败')
                      }
                    }}
                  >
                    恢复默认
                  </Button>
                </Space>
              </Form>
            </Card>
          ) : null}
        </Layout.Content>
      </Layout>
    </AdminLayout>
  )
}

function RuntimeActions({
  saving,
  onReload,
}: {
  saving: boolean
  onReload: () => Promise<void>
}) {
  return (
    <Space wrap>
      <Button type="primary" htmlType="submit" loading={saving}>
        保存并热加载
      </Button>
      <Button icon={<RefreshCw {...icon} />} onClick={() => void onReload()}>
        重新加载
      </Button>
    </Space>
  )
}
