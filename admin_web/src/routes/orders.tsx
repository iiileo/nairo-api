import { createFileRoute, redirect } from '@tanstack/react-router'
import { Pencil, RefreshCw } from 'lucide-react'
import { App, Button, Card, Form, Input, Select, Table, Tag, Typography } from 'antd'
import { useEffect, useState } from 'react'
import { AdminLayout } from '@/components/AdminLayout'
import { ApiError, api, type OrderItem } from '@/lib/api'
import { getToken } from '@/lib/auth'
import { fenToYuan, statusLabel } from '@/lib/format'

const icon = { size: 14, strokeWidth: 2 }

export const Route = createFileRoute('/orders')({
  beforeLoad: () => {
    if (!getToken()) throw redirect({ to: '/login' })
  },
  component: OrdersPage,
})

function OrdersPage() {
  const { message, modal } = App.useApp()
  const [status, setStatus] = useState<string | undefined>('pending')
  const [items, setItems] = useState<OrderItem[]>([])
  const [loading, setLoading] = useState(false)
  const [form] = Form.useForm()

  async function load(nextStatus = status) {
    setLoading(true)
    try {
      const res = await api.orders(nextStatus)
      setItems(res.items)
    } catch (err) {
      message.error(err instanceof ApiError ? '加载失败' : '网络错误')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [status])

  function openStatus(row: OrderItem) {
    form.setFieldsValue({
      status: 'paid',
      remark: '用户已付款但 App 未上报，人工确认',
      channel: row.channel,
    })
    modal.confirm({
      title: '修改订单状态',
      width: 480,
      content: (
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="status" label="目标状态" rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'paid', label: '已支付' },
                { value: 'closed', label: '已关闭' },
                { value: 'expired', label: '已过期' },
              ]}
            />
          </Form.Item>
          <Form.Item name="remark" label="备注" rules={[{ required: true }]}>
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="channel" label="渠道">
            <Select
              options={[
                { value: 'wechat', label: '微信' },
                { value: 'alipay', label: '支付宝' },
              ]}
            />
          </Form.Item>
        </Form>
      ),
      onOk: async () => {
        const values = await form.validateFields()
        await api.setOrderStatus(row.id, values)
        message.success('状态已更新')
        await load()
      },
    })
  }

  return (
    <AdminLayout
      extra={
        <>
          <Select
            allowClear
            placeholder="状态筛选"
            style={{ width: 140 }}
            value={status}
            onChange={(v) => setStatus(v)}
            options={[
              { value: 'pending', label: '待支付' },
              { value: 'paid', label: '已支付' },
              { value: 'expired', label: '已过期' },
              { value: 'closed', label: '已关闭' },
            ]}
          />
          <Button icon={<RefreshCw {...icon} />} onClick={() => void load()}>
            刷新
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
          scroll={{ x: 1180 }}
          pagination={{ pageSize: 20, showSizeChanger: true, style: { paddingInline: 16 } }}
          columns={[
            {
              title: '商户单号',
              dataIndex: 'out_trade_no',
              width: 260,
              render: (v: string) => (
                <Typography.Text
                  copyable={{ text: v }}
                  style={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}
                >
                  {v}
                </Typography.Text>
              ),
            },
            {
              title: '订单 ID',
              dataIndex: 'id',
              width: 210,
              render: (v: string) => (
                <Typography.Text
                  copyable={{ text: v }}
                  style={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}
                >
                  {v}
                </Typography.Text>
              ),
            },
            {
              title: '用户',
              dataIndex: 'user_id',
              width: 210,
              render: (v: string) =>
                v ? (
                  <Typography.Text
                    copyable={{ text: v }}
                    style={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}
                  >
                    {v}
                  </Typography.Text>
                ) : (
                  '-'
                ),
            },
            {
              title: '标价',
              dataIndex: 'amount_cents',
              width: 96,
              align: 'right',
              render: (v: number) => `¥${fenToYuan(v)}`,
            },
            {
              title: '实付',
              dataIndex: 'pay_amount_cents',
              width: 96,
              align: 'right',
              render: (v: number) => `¥${fenToYuan(v)}`,
            },
            {
              title: '渠道',
              dataIndex: 'channel',
              width: 80,
              render: (v: string) => statusLabel(v),
            },
            {
              title: '状态',
              dataIndex: 'status',
              width: 88,
              render: (v: string) => <Tag>{statusLabel(v)}</Tag>,
            },
            {
              title: '入账',
              dataIndex: 'settle_type',
              width: 72,
              render: (v: string) => statusLabel(v),
            },
            {
              title: '操作',
              key: 'actions',
              fixed: 'right',
              width: 96,
              render: (_, row) =>
                row.status === 'pending' || row.status === 'expired' ? (
                  <Button type="link" size="small" icon={<Pencil {...icon} />} onClick={() => openStatus(row)}>
                    改状态
                  </Button>
                ) : (
                  <span style={{ color: 'rgba(0,0,0,0.25)' }}>—</span>
                ),
            },
          ]}
        />
      </Card>
    </AdminLayout>
  )
}
