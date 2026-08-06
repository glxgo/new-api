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
import { useDeferredValue, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Check, Loader2, Search, UserPlus, Users } from 'lucide-react'
import { toast } from 'sonner'
import { formatQuotaAsUSD } from '@/lib/currency'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Dialog } from '@/components/dialog'
import { searchUsers } from '@/features/users/api'
import type { User } from '@/features/users/types'
import {
  getAdminVirtualMembershipPlans,
  grantAdminVirtualMembership,
} from '@/features/virtual-membership/api'

function quotaText(value: number) {
  return formatQuotaAsUSD(value, {
    digitsLarge: 2,
    digitsSmall: 4,
    abbreviate: true,
  })
}

export function GrantMembershipDialog({
  open,
  onOpenChange,
  onSuccess,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: () => void | Promise<void>
}) {
  const [userQuery, setUserQuery] = useState('')
  const deferredUserQuery = useDeferredValue(userQuery.trim())
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [selectedPlanId, setSelectedPlanId] = useState('')
  const [groupSize, setGroupSize] = useState('1')
  const [submitting, setSubmitting] = useState(false)
  const plansQuery = useQuery({
    queryKey: ['admin-virtual-membership-plans'],
    queryFn: getAdminVirtualMembershipPlans,
    enabled: open,
  })
  const usersQuery = useQuery({
    queryKey: ['admin-user-search', deferredUserQuery],
    queryFn: () =>
      searchUsers({ keyword: deferredUserQuery, p: 1, page_size: 8 }),
    enabled: open && deferredUserQuery.length > 0 && !selectedUser,
  })
  const plans = (plansQuery.data?.data ?? []).filter((plan) => plan.enabled)
  const users = usersQuery.data?.data?.items ?? []
  const selectedPlan = plans.find((plan) => String(plan.id) === selectedPlanId)
  const selectedVariant = selectedPlan?.variants.find(
    (variant) => variant.group_size === Number(groupSize)
  )

  const reset = () => {
    setUserQuery('')
    setSelectedUser(null)
    setSelectedPlanId('')
    setGroupSize('1')
    setSubmitting(false)
  }
  const changeOpen = (nextOpen: boolean) => {
    if (!nextOpen) reset()
    onOpenChange(nextOpen)
  }
  const submit = async () => {
    if (!selectedUser || !selectedPlan || !selectedVariant) {
      toast.error('请选择用户、会员方案和购买档位')
      return
    }
    setSubmitting(true)
    try {
      const result = await grantAdminVirtualMembership({
        user_id: selectedUser.id,
        plan_id: selectedPlan.id,
        group_size: selectedVariant.group_size,
      })
      if (!result.success) {
        toast.error(result.message || '添加虚拟会员失败')
        return
      }
      toast.success(
        `已为 ${selectedUser.display_name || selectedUser.username} 添加 ${selectedPlan.title}`
      )
      await onSuccess()
      changeOpen(false)
    } catch {
      // The shared API interceptor presents the backend error.
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={changeOpen}
      title={
        <span className='flex items-center gap-2'>
          <UserPlus className='size-5 text-emerald-600' />
          手动添加虚拟会员
        </span>
      }
      description='直接为用户发放 Plus、Pro 5x 或 Pro 20x 等已启用方案，不扣余额、不进入支付。'
      contentClassName='sm:max-w-xl'
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => changeOpen(false)}
            disabled={submitting}
          >
            取消
          </Button>
          <Button
            onClick={() => void submit()}
            disabled={
              submitting || !selectedUser || !selectedPlan || !selectedVariant
            }
          >
            {submitting ? (
              <Loader2 className='size-4 animate-spin' />
            ) : (
              <UserPlus className='size-4' />
            )}
            确认添加
          </Button>
        </>
      }
    >
      <div className='space-y-5'>
        <section>
          <div className='mb-2 flex items-center justify-between'>
            <label className='text-sm font-medium'>1. 选择用户</label>
            {selectedUser && (
              <Button
                variant='ghost'
                size='xs'
                onClick={() => setSelectedUser(null)}
              >
                重新选择
              </Button>
            )}
          </div>
          {selectedUser ? (
            <div className='flex items-center justify-between rounded-xl border border-emerald-500/30 bg-emerald-500/5 p-3'>
              <div>
                <p className='font-medium'>
                  {selectedUser.display_name || selectedUser.username}
                </p>
                <p className='text-muted-foreground text-xs'>
                  @{selectedUser.username} · ID {selectedUser.id}
                  {selectedUser.email ? ` · ${selectedUser.email}` : ''}
                </p>
              </div>
              <Check className='size-5 text-emerald-600' />
            </div>
          ) : (
            <>
              <div className='relative'>
                <Search className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2' />
                <Input
                  value={userQuery}
                  onChange={(event) => setUserQuery(event.target.value)}
                  className='pl-9'
                  placeholder='输入用户名、邮箱或用户 ID'
                />
              </div>
              {deferredUserQuery && (
                <div className='mt-2 max-h-44 space-y-1 overflow-y-auto rounded-xl border p-1.5'>
                  {usersQuery.isFetching ? (
                    <div className='text-muted-foreground flex items-center justify-center gap-2 py-6 text-xs'>
                      <Loader2 className='size-4 animate-spin' />
                      正在搜索用户
                    </div>
                  ) : users.length === 0 ? (
                    <p className='text-muted-foreground py-6 text-center text-xs'>
                      没有找到用户
                    </p>
                  ) : (
                    users.map((user) => (
                      <button
                        key={user.id}
                        type='button'
                        disabled={user.status !== 1}
                        onClick={() => setSelectedUser(user)}
                        className={cn(
                          'hover:bg-muted flex w-full items-center justify-between rounded-lg px-3 py-2 text-left transition-colors',
                          user.status !== 1 && 'cursor-not-allowed opacity-50'
                        )}
                      >
                        <span>
                          <span className='block text-sm font-medium'>
                            {user.display_name || user.username}
                          </span>
                          <span className='text-muted-foreground block text-[10px]'>
                            @{user.username} · ID {user.id}
                          </span>
                        </span>
                        <Users className='text-muted-foreground size-4' />
                      </button>
                    ))
                  )}
                </div>
              )}
            </>
          )}
        </section>

        <section className='grid gap-3 sm:grid-cols-2'>
          <label className='text-sm font-medium'>
            <span className='mb-2 block'>2. 会员方案</span>
            <Select
              value={selectedPlanId}
              onValueChange={(value) => setSelectedPlanId(value ?? '')}
            >
              <SelectTrigger className='w-full'>
                <SelectValue placeholder='选择 Plus / Pro 方案' />
              </SelectTrigger>
              <SelectContent>
                {plans.map((plan) => (
                  <SelectItem key={plan.id} value={String(plan.id)}>
                    {plan.title}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </label>
          <label className='text-sm font-medium'>
            <span className='mb-2 block'>3. 会员档位</span>
            <Select
              value={groupSize}
              onValueChange={(value) => setGroupSize(value ?? '1')}
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {[1, 2, 3, 4].map((size) => (
                  <SelectItem key={size} value={String(size)}>
                    {size === 1 ? '单独购买档' : `${size} 人档`}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </label>
        </section>

        {selectedPlan && selectedVariant && (
          <div className='bg-muted/40 rounded-xl border p-4'>
            <div className='flex items-center justify-between gap-3'>
              <div>
                <p className='font-semibold'>{selectedPlan.title}</p>
                <p className='text-muted-foreground text-xs'>
                  {selectedVariant.label} · 有效期 {selectedPlan.duration_days}{' '}
                  天
                </p>
              </div>
              <span className='font-semibold text-emerald-600'>免费发放</span>
            </div>
            <div className='mt-3 grid grid-cols-2 gap-2 text-xs sm:grid-cols-4'>
              <div>
                <p className='text-muted-foreground'>周额度</p>
                <p className='mt-0.5 font-medium'>
                  {quotaText(selectedVariant.weekly_quota)}
                </p>
              </div>
              <div>
                <p className='text-muted-foreground'>5 小时额度</p>
                <p className='mt-0.5 font-medium'>
                  {selectedPlan.five_hour_enabled
                    ? quotaText(selectedVariant.five_hour_quota)
                    : '未开启'}
                </p>
              </div>
              <div>
                <p className='text-muted-foreground'>并发</p>
                <p className='mt-0.5 font-medium'>
                  {selectedVariant.concurrency_limit || '不限'}
                </p>
              </div>
              <div>
                <p className='text-muted-foreground'>RPM</p>
                <p className='mt-0.5 font-medium'>
                  {selectedVariant.rpm_limit || '不限'}
                </p>
              </div>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  )
}
