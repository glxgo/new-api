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
import { useState } from 'react'
import { EyeOff, Gauge } from 'lucide-react'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/dialog'
import { setSelfVirtualMembershipHidden } from '../api'
import type { UserVirtualMembership } from '../types'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  membership: UserVirtualMembership | null
  onHidden?: () => void | Promise<void>
}

export function VirtualMembershipManagementDialog({
  open,
  onOpenChange,
  membership,
  onHidden,
}: Props) {
  const [hiding, setHiding] = useState(false)

  const hideMembership = async () => {
    if (!membership?.id) return
    setHiding(true)
    try {
      const response = await setSelfVirtualMembershipHidden(membership.id, true)
      if (!response.success) {
        toast.error(response.message || '隐藏失败')
        return
      }
      toast.success('已隐藏该会员，可联系管理员恢复展示')
      await onHidden?.()
      onOpenChange(false)
    } catch {
      toast.error('隐藏失败，请稍后重试')
    } finally {
      setHiding(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={
        <span className='flex items-center gap-2'>
          <Gauge className='h-5 w-5' />
          管理虚拟会员
        </span>
      }
      description={`${membership?.plan_title || '虚拟会员'} #${membership?.id || ''}`}
      contentClassName='sm:max-w-md'
      footer={
        <Button variant='outline' onClick={() => onOpenChange(false)}>
          关闭
        </Button>
      }
    >
      <div className='space-y-4'>
        {membership && (
          <div className='bg-muted/40 space-y-2 rounded-xl border p-4 text-sm'>
            <div className='flex items-center justify-between gap-3'>
              <span className='text-muted-foreground'>方案</span>
              <span className='font-medium'>{membership.plan_title}</span>
            </div>
            <div className='flex items-center justify-between gap-3'>
              <span className='text-muted-foreground'>购买档位</span>
              <span className='font-medium'>
                {membership.group_size === 1
                  ? '单独购买'
                  : `${membership.group_size} 人团`}
              </span>
            </div>
            <div className='flex items-center justify-between gap-3'>
              <span className='text-muted-foreground'>状态</span>
              <span className='font-medium'>{membership.status}</span>
            </div>
            <div className='flex items-center justify-between gap-3'>
              <span className='text-muted-foreground'>有效期至</span>
              <span className='font-medium'>
                {formatTimestampToDate(membership.end_time)}
              </span>
            </div>
          </div>
        )}

        <div className='border-warning/30 bg-warning/5 flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3'>
          <div className='min-w-0'>
            <div className='text-sm font-medium'>管理剩余用量展示</div>
            <div className='text-muted-foreground text-xs'>
              只从你的剩余用量页面隐藏，不会影响额度、绑定或实际使用。
            </div>
          </div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='gap-1.5'
            onClick={hideMembership}
            disabled={hiding}
          >
            <EyeOff className='size-3.5' />
            {hiding ? '处理中…' : '隐藏此会员'}
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
