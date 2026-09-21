/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { FileText, Loader2, Mail, Receipt, Send } from 'lucide-react'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { SectionPageLayout } from '@/components/layout'
import {
  createInvoiceApplication,
  getEligibleInvoiceOrders,
  getUserInvoiceApplications,
} from './api'
import type { InvoiceApplication, InvoiceEligibleOrder } from './types'

function money(amount: number, currency: string) {
  const symbol =
    currency === 'CNY' ? '¥' : currency === 'USD' ? '$' : `${currency} `
  return `${symbol}${Number(amount || 0).toFixed(2)}`
}

function statusMeta(status: InvoiceApplication['status']) {
  if (status === 'approved')
    return {
      label: '已通过',
      className: 'text-emerald-600 bg-emerald-500/10 border-emerald-500/20',
    }
  if (status === 'rejected')
    return {
      label: '已拒绝',
      className: 'text-destructive bg-destructive/10 border-destructive/20',
    }
  return {
    label: '待审核',
    className: 'text-amber-600 bg-amber-500/10 border-amber-500/20',
  }
}

export function Invoices() {
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [subjectType, setSubjectType] = useState('company')
  const [title, setTitle] = useState('')
  const [taxpayerId, setTaxpayerId] = useState('')
  const [email, setEmail] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const ordersQuery = useQuery({
    queryKey: ['invoice-eligible-orders'],
    queryFn: getEligibleInvoiceOrders,
  })
  const applicationsQuery = useQuery({
    queryKey: ['invoice-applications-self'],
    queryFn: getUserInvoiceApplications,
  })
  const orders = ordersQuery.data?.data ?? []
  const applications = applicationsQuery.data?.data ?? []
  const selectedOrders = useMemo(
    () => orders.filter((order) => selected.has(order.id)),
    [orders, selected]
  )
  const selectedCurrency = selectedOrders[0]?.currency ?? 'CNY'
  const selectedTotal = selectedOrders.reduce(
    (sum, order) => sum + order.amount,
    0
  )
  const toggle = (order: InvoiceEligibleOrder, checked: boolean) => {
    if (
      checked &&
      selectedOrders.length > 0 &&
      order.currency !== selectedCurrency
    ) {
      toast.error('不同币种的充值订单不能合并开票')
      return
    }
    setSelected((current) => {
      const next = new Set(current)
      if (checked) next.add(order.id)
      else next.delete(order.id)
      return next
    })
  }
  const selectAllSameCurrency = () => {
    const currency = selectedOrders[0]?.currency ?? orders[0]?.currency
    if (!currency) return
    setSelected(
      new Set(
        orders
          .filter((order) => order.currency === currency)
          .map((order) => order.id)
      )
    )
  }
  const submit = async () => {
    if (selectedOrders.length === 0)
      return toast.error('请选择至少一笔充值订单')
    if (subjectType === 'company' && !taxpayerId.trim())
      return toast.error('企业主体请填写纳税人识别号')
    if (!title.trim() || !email.trim())
      return toast.error('请填写发票抬头和接收邮箱')
    setSubmitting(true)
    try {
      const result = await createInvoiceApplication({
        top_up_ids: selectedOrders.map((order) => order.id),
        subject_type: subjectType,
        title: title.trim(),
        taxpayer_id: taxpayerId.trim(),
        email: email.trim(),
      })
      if (!result.success) {
        toast.error(result.message || '提交发票申请失败')
        return
      }
      toast.success('发票申请已提交，开票后会通知你')
      setSelected(new Set())
      setTitle('')
      setTaxpayerId('')
      await queryClient.invalidateQueries({
        queryKey: ['invoice-eligible-orders'],
      })
      await queryClient.invalidateQueries({
        queryKey: ['invoice-applications-self'],
      })
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <SectionPageLayout variant='editorial'>
      <SectionPageLayout.Title>发票中心</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        选择已支付成功的充值订单申请发票，订单金额可合并但每笔订单只能开票一次。
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-7xl flex-col gap-5'>
          <section className='relative overflow-hidden rounded-2xl border bg-[radial-gradient(circle_at_85%_15%,rgba(37,99,235,.14),transparent_35%),linear-gradient(145deg,var(--card),color-mix(in_oklch,var(--card)_92%,var(--muted)))] p-5 sm:p-6'>
            <div className='pointer-events-none absolute inset-0 [background-image:linear-gradient(to_right,currentColor_1px,transparent_1px),linear-gradient(to_bottom,currentColor_1px,transparent_1px)] [background-size:24px_24px] opacity-[0.05]' />
            <div className='relative flex items-start gap-3'>
              <span className='flex size-11 shrink-0 items-center justify-center rounded-xl border border-blue-500/20 bg-blue-500/10 text-blue-600'>
                <Receipt className='size-5' />
              </span>
              <div>
                <h2 className='font-serif text-2xl font-semibold tracking-[-0.03em]'>
                  开票申请
                </h2>
                <p className='text-muted-foreground mt-1 max-w-2xl text-sm leading-6'>
                  仅支持已支付成功的充值订单。提交后，运营人员会按订单实际支付金额人工开票。
                </p>
              </div>
            </div>
          </section>

          <div className='grid gap-5 xl:grid-cols-[minmax(0,1.35fr)_minmax(20rem,.65fr)]'>
            <section className='bg-card overflow-hidden rounded-2xl border'>
              <header className='flex items-center justify-between gap-3 border-b px-4 py-4 sm:px-5'>
                <div>
                  <h3 className='font-semibold'>可开票订单</h3>
                  <p className='text-muted-foreground mt-1 text-xs'>
                    已选择 {selectedOrders.length} 笔 · 合计{' '}
                    {money(selectedTotal, selectedCurrency)}
                  </p>
                </div>
                <Button
                  variant='ghost'
                  size='sm'
                  onClick={selectAllSameCurrency}
                  disabled={orders.length === 0}
                >
                  全选同币种
                </Button>
              </header>
              <div className='max-h-[30rem] overflow-y-auto'>
                {ordersQuery.isLoading ? (
                  <div className='text-muted-foreground flex min-h-44 items-center justify-center text-sm'>
                    加载中…
                  </div>
                ) : orders.length === 0 ? (
                  <div className='text-muted-foreground flex min-h-44 flex-col items-center justify-center gap-2 text-sm'>
                    <FileText className='size-7 opacity-35' />
                    暂无可开票充值订单
                  </div>
                ) : (
                  <div className='divide-y'>
                    {orders.map((order) => (
                      <label
                        key={order.id}
                        className={cn(
                          'hover:bg-muted/40 flex cursor-pointer items-center gap-3 px-4 py-3 transition-colors sm:px-5',
                          selected.has(order.id) && 'bg-blue-500/5'
                        )}
                      >
                        <Checkbox
                          checked={selected.has(order.id)}
                          onCheckedChange={(checked) =>
                            toggle(order, checked === true)
                          }
                        />
                        <span className='min-w-0 flex-1'>
                          <span className='block truncate font-mono text-xs font-semibold'>
                            {order.trade_no}
                          </span>
                          <span className='text-muted-foreground mt-1 block text-xs'>
                            {dayjs(order.payment_time * 1000).format(
                              'YYYY-MM-DD HH:mm'
                            )}{' '}
                            · {order.payment_method || '在线支付'}
                          </span>
                        </span>
                        <span className='shrink-0 font-mono text-sm font-semibold'>
                          {money(order.amount, order.currency)}
                        </span>
                      </label>
                    ))}
                  </div>
                )}
              </div>
            </section>

            <section className='bg-card rounded-2xl border p-4 sm:p-5'>
              <div className='flex items-center gap-2'>
                <Mail className='size-4 text-blue-600' />
                <h3 className='font-semibold'>发票信息</h3>
              </div>
              <div className='mt-4 space-y-4'>
                <div>
                  <Label>主体类型</Label>
                  <Select
                    value={subjectType}
                    onValueChange={(value) => value && setSubjectType(value)}
                  >
                    <SelectTrigger className='mt-1.5 w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='company'>企业</SelectItem>
                      <SelectItem value='personal'>个人</SelectItem>
                      <SelectItem value='other'>其他</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label htmlFor='invoice-title'>抬头名称</Label>
                  <Input
                    id='invoice-title'
                    className='mt-1.5'
                    value={title}
                    onChange={(event) => setTitle(event.target.value)}
                    placeholder='请输入发票抬头'
                  />
                </div>
                <div>
                  <Label htmlFor='invoice-taxpayer'>
                    纳税人识别号{' '}
                    <span className='text-muted-foreground font-normal'>
                      （个人可不填）
                    </span>
                  </Label>
                  <Input
                    id='invoice-taxpayer'
                    className='mt-1.5'
                    value={taxpayerId}
                    onChange={(event) => setTaxpayerId(event.target.value)}
                    placeholder='企业主体必填'
                  />
                </div>
                <div>
                  <Label htmlFor='invoice-email'>发票接收邮箱</Label>
                  <Input
                    id='invoice-email'
                    type='email'
                    className='mt-1.5'
                    value={email}
                    onChange={(event) => setEmail(event.target.value)}
                    placeholder='invoice@example.com'
                  />
                </div>
                <div className='rounded-xl border border-blue-500/20 bg-blue-500/5 p-3'>
                  <div className='flex items-center justify-between gap-3 text-sm'>
                    <span className='text-muted-foreground'>发票金额</span>
                    <strong className='text-lg'>
                      {money(selectedTotal, selectedCurrency)}
                    </strong>
                  </div>
                  <p className='text-muted-foreground mt-1 text-[11px]'>
                    金额严格等于所选订单合计，不可增加。
                  </p>
                </div>
                <Button
                  className='w-full'
                  onClick={() => void submit()}
                  disabled={submitting || selectedOrders.length === 0}
                >
                  {submitting ? (
                    <Loader2 className='size-4 animate-spin' />
                  ) : (
                    <Send className='size-4' />
                  )}
                  提交开票申请
                </Button>
              </div>
            </section>
          </div>

          <section className='bg-card overflow-hidden rounded-2xl border'>
            <header className='border-b px-4 py-4 sm:px-5'>
              <h3 className='font-semibold'>我的申请</h3>
              <p className='text-muted-foreground mt-1 text-xs'>
                审核通过或拒绝后，你会收到站内配置的通知及邮件。
              </p>
            </header>
            {applicationsQuery.isLoading ? (
              <div className='text-muted-foreground p-6 text-sm'>加载中…</div>
            ) : applications.length === 0 ? (
              <div className='text-muted-foreground p-8 text-center text-sm'>
                暂无发票申请记录
              </div>
            ) : (
              <div className='divide-y'>
                {applications.map((application) => {
                  const meta = statusMeta(application.status)
                  return (
                    <article key={application.id} className='px-4 py-4 sm:px-5'>
                      <div className='flex flex-wrap items-center justify-between gap-2'>
                        <div className='flex items-center gap-2'>
                          <span className='font-mono text-sm font-semibold'>
                            申请 #{application.id}
                          </span>
                          <span
                            className={cn(
                              'rounded-full border px-2 py-0.5 text-[11px] font-medium',
                              meta.className
                            )}
                          >
                            {meta.label}
                          </span>
                        </div>
                        <span className='text-muted-foreground text-xs'>
                          {dayjs(application.created_at * 1000).format(
                            'YYYY-MM-DD HH:mm'
                          )}
                        </span>
                      </div>
                      <div className='mt-3 grid gap-2 text-xs sm:grid-cols-4'>
                        <span>
                          <b className='text-muted-foreground font-normal'>
                            抬头：
                          </b>
                          {application.title}
                        </span>
                        <span>
                          <b className='text-muted-foreground font-normal'>
                            主体：
                          </b>
                          {application.subject_label}
                        </span>
                        <span>
                          <b className='text-muted-foreground font-normal'>
                            金额：
                          </b>
                          {money(
                            application.total_amount,
                            application.currency
                          )}
                        </span>
                        <span>
                          <b className='text-muted-foreground font-normal'>
                            订单：
                          </b>
                          {application.orders.length} 笔
                        </span>
                      </div>
                      {application.remark && (
                        <p className='text-muted-foreground bg-muted/40 mt-2 rounded-lg px-3 py-2 text-xs'>
                          审核备注：{application.remark}
                        </p>
                      )}
                    </article>
                  )
                })}
              </div>
            )}
          </section>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
