import { createFileRoute, redirect } from '@tanstack/react-router'
import { Ellipsis, KeyRound, Pencil, Percent, Plus, RefreshCw, Wallet } from 'lucide-react'
import {
  App,
  Button,
  Card,
  Dropdown,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd'
import type { MenuProps } from 'antd'
import { useEffect, useState } from 'react'
import { AdminLayout } from '@/components/AdminLayout'
import { ApiError, api, type UserItem } from '@/lib/api'
import { getToken } from '@/lib/auth'
import { yuanToFen } from '@/lib/format'

const icon = { size: 14, strokeWidth: 2 }

export const Route = createFileRoute('/')({
  beforeLoad: () => {
    if (!getToken()) throw redirect({ to: '/login' })
  },
  component: UsersPage,
})

function UsersPage() {
  const { message, modal } = App.useApp()
  const [items, setItems] = useState<UserItem[]>([])
  const [loading, setLoading] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [pwdOpen, setPwdOpen] = useState(false)
  const [current, setCurrent] = useState<UserItem | null>(null)
  const [createForm] = Form.useForm()
  const [editForm] = Form.useForm()
  const [pwdForm] = Form.useForm()
  const [feeForm] = Form.useForm()
  const [rechargeForm] = Form.useForm()

  async function load() {
    setLoading(true)
    try {
      const res = await api.users(100, 0)
      setItems(res.items)
    } catch (err) {
      message.error(err instanceof ApiError ? '加载失败' : '网络错误')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  function openEdit(row: UserItem) {
    setCurrent(row)
    editForm.setFieldsValue({
      email: row.email || '',
      phone: row.phone || '',
      status: row.status,
    })
    setEditOpen(true)
  }

  function openPassword(row: UserItem) {
    setCurrent(row)
    pwdForm.resetFields()
    setPwdOpen(true)
  }

  function openFee(row: UserItem) {
    feeForm.setFieldsValue({ fee_rate_bps: row.fee_rate_bps })
    modal.confirm({
      title: '调整费率',
      content: (
        <Form form={feeForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="fee_rate_bps" label="费率（100 表示 1%）" rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      ),
      onOk: async () => {
        const values = await feeForm.validateFields()
        await api.setFee(row.id, values.fee_rate_bps)
        message.success('已更新费率')
        await load()
      },
    })
  }

  function openRecharge(row: UserItem) {
    rechargeForm.setFieldsValue({ amount_yuan: 100, remark: '后台充值' })
    modal.confirm({
      title: '手续费充值',
      content: (
        <Form form={rechargeForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="amount_yuan" label="金额（元）" rules={[{ required: true }]}>
            <InputNumber min={0.01} step={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input />
          </Form.Item>
        </Form>
      ),
      onOk: async () => {
        const values = await rechargeForm.validateFields()
        await api.recharge(row.id, yuanToFen(values.amount_yuan), values.remark)
        message.success('充值成功')
      },
    })
  }

  function moreMenu(row: UserItem): MenuProps['items'] {
    return [
      {
        key: 'fee',
        icon: <Percent {...icon} />,
        label: '调整费率',
        onClick: () => openFee(row),
      },
      {
        key: 'recharge',
        icon: <Wallet {...icon} />,
        label: '手续费充值',
        onClick: () => openRecharge(row),
      },
    ]
  }

  return (
    <AdminLayout
      extra={
        <>
          <Button icon={<RefreshCw {...icon} />} onClick={() => void load()}>
            刷新
          </Button>
          <Button type="primary" icon={<Plus {...icon} />} onClick={() => setCreateOpen(true)}>
            新建商户
          </Button>
        </>
      }
    >
      <Card styles={{ body: { padding: 0 } }}>
        <Table
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={items}
          scroll={{ x: 980 }}
          pagination={{ pageSize: 20, showSizeChanger: true, style: { paddingInline: 16 } }}
          columns={[
            {
              title: 'ID',
              dataIndex: 'id',
              width: 210,
              render: (v: string) => (
                <Typography.Text copyable={{ text: v }} style={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}>
                  {v}
                </Typography.Text>
              ),
            },
            {
              title: '邮箱',
              dataIndex: 'email',
              width: 220,
              render: (v) => v || '-',
            },
            {
              title: '手机',
              dataIndex: 'phone',
              width: 140,
              render: (v) => v || '-',
            },
            {
              title: '费率',
              dataIndex: 'fee_rate_bps',
              width: 88,
              align: 'right',
              render: (v: number) => `${(v / 100).toFixed(2)}%`,
            },
            {
              title: '状态',
              dataIndex: 'status',
              width: 88,
              render: (v: string) => (
                <Tag color={v === 'active' ? 'success' : v === 'disabled' ? 'error' : 'default'}>
                  {v === 'active' ? '正常' : v === 'disabled' ? '停用' : v}
                </Tag>
              ),
            },
            {
              title: '操作',
              key: 'actions',
              fixed: 'right',
              width: 168,
              render: (_, row) => (
                <Space size={0}>
                  <Button type="link" size="small" icon={<Pencil {...icon} />} onClick={() => openEdit(row)}>
                    编辑
                  </Button>
                  <Button
                    type="link"
                    size="small"
                    icon={<KeyRound {...icon} />}
                    onClick={() => openPassword(row)}
                  >
                    密码
                  </Button>
                  <Dropdown menu={{ items: moreMenu(row) }}>
                    <Button type="link" size="small" icon={<Ellipsis {...icon} />}>
                      更多
                    </Button>
                  </Dropdown>
                </Space>
              ),
            },
          ]}
        />
      </Card>

      <Modal
        title="新建商户"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={() => createForm.submit()}
        destroyOnHidden
        okText="创建"
      >
        <Form
          form={createForm}
          layout="vertical"
          initialValues={{ fee_rate_bps: 100 }}
          onFinish={async (values) => {
            try {
              const res = await api.createUser(values)
              setCreateOpen(false)
              createForm.resetFields()
              modal.success({
                title: '商户已创建',
                content: (
                  <div>
                    <p>Access Key: {res.access_key}</p>
                    <p>API Secret: {res.api_secret}</p>
                    <Typography.Text type="warning">请立即保存 Secret，之后无法再次查看。</Typography.Text>
                  </div>
                ),
              })
              await load()
            } catch (err) {
              message.error(err instanceof ApiError ? err.code : '创建失败')
            }
          }}
        >
          <Form.Item name="email" label="邮箱">
            <Input placeholder="可用于登录" />
          </Form.Item>
          <Form.Item name="phone" label="手机号">
            <Input placeholder="可用于登录" />
          </Form.Item>
          <Form.Item name="password" label="初始密码" rules={[{ required: true, min: 6 }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="fee_rate_bps" label="费率（100 表示 1%）" rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="修改账号"
        open={editOpen}
        onCancel={() => setEditOpen(false)}
        onOk={() => editForm.submit()}
        destroyOnHidden
        okText="保存"
      >
        <Form
          form={editForm}
          layout="vertical"
          onFinish={async (values) => {
            if (!current) return
            try {
              await api.updateUser(current.id, {
                email: values.email ?? '',
                phone: values.phone ?? '',
                status: values.status,
              })
              message.success('账号已更新')
              setEditOpen(false)
              await load()
            } catch (err) {
              const code = err instanceof ApiError ? err.code : ''
              message.error(code === 'email_taken' ? '邮箱已被占用' : '更新失败')
            }
          }}
        >
          <Form.Item name="email" label="邮箱">
            <Input placeholder="留空则清除邮箱" />
          </Form.Item>
          <Form.Item name="phone" label="手机号">
            <Input placeholder="留空则清除手机号" />
          </Form.Item>
          <Form.Item name="status" label="状态" rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'active', label: '正常' },
                { value: 'disabled', label: '停用' },
                { value: 'pending', label: '待审核' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="修改密码"
        open={pwdOpen}
        onCancel={() => setPwdOpen(false)}
        onOk={() => pwdForm.submit()}
        destroyOnHidden
        okText="保存"
      >
        <Form
          form={pwdForm}
          layout="vertical"
          onFinish={async (values) => {
            if (!current) return
            try {
              await api.setPassword(current.id, values.password)
              message.success('密码已更新')
              setPwdOpen(false)
            } catch (err) {
              message.error(err instanceof ApiError ? '密码不符合要求' : '更新失败')
            }
          }}
        >
          <Typography.Paragraph type="secondary">
            用户：{current?.email || current?.phone || current?.id}
          </Typography.Paragraph>
          <Form.Item
            name="password"
            label="新密码"
            rules={[{ required: true, min: 6, message: '至少 6 位' }]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            name="confirm"
            label="确认密码"
            dependencies={['password']}
            rules={[
              { required: true, message: '请再次输入密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('password') === value) return Promise.resolve()
                  return Promise.reject(new Error('两次输入不一致'))
                },
              }),
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>
    </AdminLayout>
  )
}
