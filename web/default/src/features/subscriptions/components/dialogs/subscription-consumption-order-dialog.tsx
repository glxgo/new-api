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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { GripVertical, KeyRound, ListOrdered } from 'lucide-react'
import { Reorder } from 'motion/react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  getSubscriptionConsumptionOrder,
  updateSubscriptionConsumptionOrder,
} from '../../api'
import type { UserSubscriptionRecord } from '../../types'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  subscriptions: UserSubscriptionRecord[]
}

export function SubscriptionConsumptionOrderDialog({
  open,
  onOpenChange,
  subscriptions,
}: Props) {
  const groups = useMemo(() => {
    const values = new Set<string>(['default'])
    subscriptions.forEach(({ subscription }) => {
      if (subscription.allowed_group) values.add(subscription.allowed_group)
    })
    return Array.from(values)
  }, [subscriptions])
  const [group, setGroup] = useState(groups[0] || 'default')
  const [revision, setRevision] = useState(0)
  const [items, setItems] = useState<
    Array<{ id: number; title: string; endTime: number; remaining: number }>
  >([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const response = await getSubscriptionConsumptionOrder(group)
      if (!response.success || !response.data) return
      const rank = new Map(
        response.data.order.map((row) => [row.subscription_id, row.priority])
      )
      setRevision(response.data.revision)
      setItems(
        response.data.subscriptions
          .map((subscription) => ({
            id: subscription.id,
            title: subscription.plan_title || `订阅实例 #${subscription.id}`,
            endTime: subscription.end_time,
            remaining: Math.max(
              0,
              Number(subscription.amount_total || 0) -
                Number(subscription.amount_used || 0)
            ),
          }))
          .sort((a, b) => {
            const ar = rank.get(a.id)
            const br = rank.get(b.id)
            if (ar !== undefined && br === undefined) return -1
            if (ar === undefined && br !== undefined) return 1
            if (ar !== undefined && br !== undefined && ar !== br)
              return ar - br
            return a.endTime - b.endTime || a.id - b.id
          })
      )
    } finally {
      setLoading(false)
    }
  }, [group])

  useEffect(() => {
    if (!open) return
    const loadTimer = window.setTimeout(() => void load(), 0)
    return () => window.clearTimeout(loadTimer)
  }, [open, load])

  async function save() {
    setSaving(true)
    try {
      const response = await updateSubscriptionConsumptionOrder({
        group,
        revision,
        subscription_ids: items.map((item) => item.id),
      })
      if (response.success) {
        setRevision(response.data?.revision || revision + 1)
        toast.success('套餐消耗顺序已保存')
        onOpenChange(false)
      }
    } catch (error: unknown) {
      const message =
        typeof error === 'object' &&
        error &&
        'response' in error &&
        typeof error.response === 'object' &&
        error.response &&
        'data' in error.response
          ? String(
              (error.response.data as { message?: string }).message ||
                '保存失败，请刷新后重试'
            )
          : '保存失败，请刷新后重试'
      toast.error(message)
      await load()
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <ListOrdered className='text-primary size-5' />
            套餐消耗顺序
          </DialogTitle>
          <DialogDescription>
            拖动整张套餐卡片调整消耗顺序。未绑定具体实例的 API Key
            按此顺序消耗；已绑定实例的 Key 不受影响。
          </DialogDescription>
        </DialogHeader>
        <Select
          value={group}
          onValueChange={(value) => value !== null && setGroup(value)}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {groups.map((item) => (
              <SelectItem key={item} value={item}>
                {item === 'default' ? '默认分组' : item}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Reorder.Group
          axis='y'
          values={items}
          onReorder={(next) => {
            if (!loading && !saving) setItems(next)
          }}
          className='max-h-[48vh] space-y-2 overflow-y-auto pr-1'
        >
          {items.map((item, index) => (
            <Reorder.Item
              key={item.id}
              value={item}
              dragListener={!loading && !saving}
              whileDrag={{
                scale: 1.015,
                boxShadow: '0 18px 38px -18px rgba(15, 23, 42, 0.42)',
              }}
              className='bg-card flex cursor-grab touch-none items-center gap-3 rounded-xl border p-3 shadow-sm select-none active:cursor-grabbing'
              aria-label={`拖动调整 ${item.title} 的消耗顺序`}
            >
              <GripVertical className='text-muted-foreground size-5 shrink-0' />
              <span className='bg-primary/10 text-primary grid size-7 shrink-0 place-items-center rounded-full font-mono text-xs font-semibold'>
                {index + 1}
              </span>
              <div className='min-w-0 flex-1'>
                <div className='truncate text-sm font-medium'>{item.title}</div>
                <div className='text-muted-foreground mt-0.5 flex items-center gap-2 text-xs'>
                  <span>#{item.id}</span>
                  <span>
                    到期 {new Date(item.endTime * 1000).toLocaleDateString()}
                  </span>
                </div>
              </div>
            </Reorder.Item>
          ))}
          {!loading && items.length === 0 && (
            <div className='text-muted-foreground grid place-items-center rounded-xl border border-dashed py-10 text-sm'>
              当前分组没有可用套餐
            </div>
          )}
        </Reorder.Group>
        <div className='bg-muted/45 text-muted-foreground flex items-start gap-2 rounded-lg p-3 text-xs leading-relaxed'>
          <KeyRound className='mt-0.5 size-4 shrink-0' />
          调整顺序不会修改 API Key 的实例绑定，也不会移动历史消费记录。
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button
            disabled={saving || loading || items.length === 0}
            onClick={save}
          >
            {saving ? '保存中…' : '保存顺序'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
