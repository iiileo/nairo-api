import { createFileRoute, redirect } from '@tanstack/react-router'
import { Ban, Link2, RefreshCw } from 'lucide-react'
import { App, Button, Card, Form, Input, Space, Table, Tag, Typography } from 'antd'
import { useEffect, useState } from 'react'
import { AdminLayout } from '@/components/AdminLayout'
import { ApiError, api, type OrphanItem } from '@/lib/api'
import { getToken } from '@/lib/auth'
import { fenToYuan, statusLabel } from '@/lib/format'

const icon = { size: 14, strokeWidth: 2 }

export const Route = createFileRoute('/orphans')({
  beforeLoad: () => {
    if (!getToken()) throw redirect({ to: '/login' })
  },
  component: OrphansPage,
})

function OrphansPage() {
  const { message, modal } = App.useApp()
  const [items, setItems] = useState<OrphanItem[]>([])
  const [loading, setLoading] = useState(false)
  const [form] = Form.useForm()

  async function load() {
    setLoading(true)
    try {
      const res = await api.orphans()
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

  function openBind(row: OrphanItem) {
    form.setFieldsValue({ order_id: '', remark: '' })
    modal.confirm({
      title: '绑定订单',
      content: (
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="order_id" label="订单 ID" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="remark" label="备注" rules={[{ required: true }]}>
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      ),
      onOk: async () => {
        const values = await form.validateFields()
        await api.bindOrphan(row.id, values)
        message.success('绑定成功')
        await load()
      },
    })
  }

  return (
    <AdminLayout
      extra={
        <Button icon={<RefreshCw {...icon} />} onClick={() => void load()}>
          刷新
        </Button>
      }
    >
      <Card styles={{ body: { padding: 0 } }}>
        <Table
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={items}
          scroll={{ x: 1200 }}
          pagination={{ pageSize: 20, showSizeChanger: true, style: { paddingInline: 16 } }}
          columns={[
            {
              title: 'ID',
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
              title: '账户',
              dataIndex: 'account_id',
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
              title: '渠道',
              dataIndex: 'channel',
              width: 80,
              render: (v: string) => statusLabel(v),
            },
            {
              title: '金额',
              dataIndex: 'pay_amount_cents',
              width: 96,
              align: 'right',
              render: (v: number) => `¥${fenToYuan(v)}`,
            },
            {
              title: '状态',
              dataIndex: 'status',
              width: 88,
              render: (v: string) => <Tag>{statusLabel(v)}</Tag>,
            },
            {
              title: '上报时间',
              dataIndex: 'reported_at',
              width: 180,
            },
            {
              title: '操作',
              key: 'actions',
              fixed: 'right',
              width: 140,
              render: (_, row) =>
                row.status === 'open' ? (
                  <Space size={0}>
                    <Button type="link" size="small" icon={<Link2 {...icon} />} onClick={() => openBind(row)}>
                      绑定
                    </Button>
                    <Button
                      type="link"
                      size="small"
                      danger
                      icon={<Ban {...icon} />}
                      onClick={async () => {
                        await api.ignoreOrphan(row.id, '后台忽略')
                        message.success('已忽略')
                        await load()
                      }}
                    >
                      忽略
                    </Button>
                  </Space>
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
