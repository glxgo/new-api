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
import { useEffect, useState } from 'react'
import { Plus, Save, TicketPercent, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { SectionPageLayout } from '@/components/layout'
import {
  deleteTopUpCoupon,
  getTopUpCoupons,
  saveTopUpCoupon,
  type TopUpCoupon,
} from './api'

const emptyCoupon: Partial<TopUpCoupon> = {
  code: '',
  title: '',
  description: '',
  discount: 0.95,
  user_limit: 1,
  enabled: true,
}

export function TopUpCoupons() {
  const [coupons, setCoupons] = useState<TopUpCoupon[]>([])
  const [editing, setEditing] = useState<Partial<TopUpCoupon>>(emptyCoupon)
  const [loading, setLoading] = useState(true)
  const load = async () => {
    setLoading(true)
    try {
      const result = await getTopUpCoupons()
      setCoupons(result.data ?? [])
    } finally {
      setLoading(false)
    }
  }
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load()
  }, [])
  const update = (field: keyof TopUpCoupon, value: string | number | boolean) =>
    setEditing((current) => ({ ...current, [field]: value }))
  const save = async () => {
    const result = await saveTopUpCoupon(editing)
    if (!result.success) return
    toast.success('优惠码已保存')
    setEditing({ ...emptyCoupon })
    await load()
  }
  const remove = async (coupon: TopUpCoupon) => {
    if (!window.confirm(`确认删除优惠码 ${coupon.code} 吗？`)) return
    const result = await deleteTopUpCoupon(coupon.id)
    if (!result.success) return
    toast.success('优惠码已删除')
    if (editing.id === coupon.id) setEditing({ ...emptyCoupon })
    await load()
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>充值优惠码</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          onClick={() => setEditing({ ...emptyCoupon })}
        >
          <Plus className='size-4' />
          新增优惠码
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='grid gap-5 xl:grid-cols-[minmax(0,1fr)_24rem]'>
          <div className='bg-card overflow-hidden rounded-2xl border'>
            <div className='border-b p-5'>
              <h2 className='flex items-center gap-2 font-semibold'>
                <TicketPercent className='size-5 text-emerald-600' />
                优惠码列表
              </h2>
              <p className='text-muted-foreground mt-1 text-xs'>
                使用次数按支付成功订单计算；待支付订单会临时占用一个名额。
              </p>
            </div>
            {loading ? (
              <p className='text-muted-foreground p-10 text-center'>加载中…</p>
            ) : coupons.length === 0 ? (
              <p className='text-muted-foreground p-10 text-center'>
                暂无优惠码
              </p>
            ) : (
              coupons.map((coupon) => (
                <div
                  key={coupon.id}
                  className='flex flex-wrap items-center justify-between gap-4 border-b p-5 last:border-b-0'
                >
                  <button
                    type='button'
                    className='min-w-0 flex-1 text-left'
                    onClick={() => setEditing(coupon)}
                  >
                    <div className='flex items-center gap-2'>
                      <span className='font-mono font-semibold'>
                        {coupon.code}
                      </span>
                      <Badge variant={coupon.enabled ? 'default' : 'outline'}>
                        {coupon.enabled ? '启用' : '停用'}
                      </Badge>
                    </div>
                    <p className='mt-1 text-sm'>{coupon.title}</p>
                    <p className='text-muted-foreground mt-1 text-xs'>
                      支付 ×{coupon.discount} · 每用户最多 {coupon.user_limit}{' '}
                      次
                    </p>
                  </button>
                  <Button
                    variant='ghost'
                    size='sm'
                    className='text-destructive'
                    onClick={() => void remove(coupon)}
                  >
                    <Trash2 className='size-4' />
                    删除
                  </Button>
                </div>
              ))
            )}
          </div>
          <div className='bg-card h-fit rounded-2xl border p-5'>
            <h2 className='font-semibold'>
              {editing.id ? '编辑' : '新增'}优惠码
            </h2>
            <div className='mt-4 space-y-3'>
              <label className='block text-xs'>
                <span className='text-muted-foreground mb-1 block'>优惠码</span>
                <Input
                  value={editing.code ?? ''}
                  onChange={(event) => update('code', event.target.value)}
                  placeholder='例如 SUMMER95'
                />
              </label>
              <label className='block text-xs'>
                <span className='text-muted-foreground mb-1 block'>
                  名称/内容
                </span>
                <Input
                  value={editing.title ?? ''}
                  onChange={(event) => update('title', event.target.value)}
                  placeholder='例如 夏日充值 95 折'
                />
              </label>
              <label className='block text-xs'>
                <span className='text-muted-foreground mb-1 block'>说明</span>
                <Textarea
                  value={editing.description ?? ''}
                  onChange={(event) =>
                    update('description', event.target.value)
                  }
                  placeholder='用户应用优惠码后看到的说明'
                />
              </label>
              <div className='grid grid-cols-2 gap-2'>
                <label className='text-xs'>
                  <span className='text-muted-foreground mb-1 block'>
                    折扣倍率
                  </span>
                  <Input
                    type='number'
                    min={0.01}
                    max={0.99}
                    step={0.01}
                    value={editing.discount ?? 0.95}
                    onChange={(event) =>
                      update('discount', Number(event.target.value))
                    }
                  />
                </label>
                <label className='text-xs'>
                  <span className='text-muted-foreground mb-1 block'>
                    每用户限次
                  </span>
                  <Input
                    type='number'
                    min={1}
                    step={1}
                    value={editing.user_limit ?? 1}
                    onChange={(event) =>
                      update('user_limit', Number(event.target.value))
                    }
                  />
                </label>
              </div>
              <label className='flex items-center gap-2 text-sm'>
                <input
                  type='checkbox'
                  checked={editing.enabled ?? true}
                  onChange={(event) => update('enabled', event.target.checked)}
                />
                启用优惠码
              </label>
              <Button className='w-full' onClick={() => void save()}>
                <Save className='size-4' />
                保存优惠码
              </Button>
            </div>
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
