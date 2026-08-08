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
import { useMemo, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  Clock3,
  Gauge,
  RefreshCw,
  Search,
  Trash2,
  UserRound,
  UserPlus,
  Users,
  WalletCards,
} from 'lucide-react'
import { toast } from 'sonner'
import { formatQuotaAsUSD } from '@/lib/currency'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  sideDrawerContentClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import {
  deleteAdminVirtualMembership,
  getAdminVirtualMemberships,
  resetAdminVirtualMemberships,
} from '@/features/virtual-membership/api'
import type { AdminVirtualMembership } from '@/features/virtual-membership/types'
import { GrantMembershipDialog } from './grant-membership-dialog'

type MembershipStatus = 'all' | 'active' | 'expired' | 'cancelled'

const statusMeta: Record<
  Exclude<MembershipStatus, 'all'>,
  { label: string; className: string }
> = {
  active: {
    label: '生效中',
    className:
      'border-emerald-500/20 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
  },
  expired: {
    label: '已过期',
    className: 'border-border bg-muted/60 text-muted-foreground',
  },
  cancelled: {
    label: '已取消',
    className: 'border-destructive/20 bg-destructive/10 text-destructive',
  },
}

function quotaText(value: number) {
  return formatQuotaAsUSD(value, {
    digitsLarge: 2,
    digitsSmall: 4,
    abbreviate: true,
  })
}

function QuotaCell({
  remaining,
  total,
  percent,
  resetAt,
  disabled = false,
}: {
  remaining: number
  total: number
  percent: number
  resetAt: number
  disabled?: boolean
}) {
  if (disabled) {
    return <span className='text-muted-foreground text-xs'>未开启</span>
  }
  return (
    <div className='min-w-40 space-y-1.5'>
      <div className='flex items-baseline justify-between gap-3 text-xs'>
        <span className='font-medium'>{quotaText(remaining)}</span>
        <span className='text-muted-foreground'>/ {quotaText(total)}</span>
      </div>
      <div className='bg-muted h-1.5 overflow-hidden rounded-full'>
        <div
          className={cn(
            'h-full rounded-full',
            percent >= 90
              ? 'bg-destructive'
              : percent >= 70
                ? 'bg-amber-500'
                : 'bg-emerald-500'
          )}
          style={{ width: `${Math.min(100, Math.max(0, percent))}%` }}
        />
      </div>
      <p className='text-muted-foreground flex items-center gap-1 text-[10px]'>
        <Clock3 className='size-3' />
        {formatTimestampToDate(resetAt)}
      </p>
    </div>
  )
}

function SummaryCard({
  icon,
  label,
  value,
}: {
  icon: ReactNode
  label: string
  value: string | number
}) {
  return (
    <div className='bg-card flex items-center gap-3 rounded-xl border px-3 py-2.5'>
      <div className='bg-muted flex size-8 items-center justify-center rounded-lg'>
        {icon}
      </div>
      <div>
        <p className='text-muted-foreground text-[10px]'>{label}</p>
        <p className='text-sm font-semibold tabular-nums'>{value}</p>
      </div>
    </div>
  )
}

export function AdminMembershipsSheet({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState<MembershipStatus>('all')
  const [grantOpen, setGrantOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] =
    useState<AdminVirtualMembership | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [resettingId, setResettingId] = useState<number | null>(null)
  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ['admin-virtual-memberships'],
    queryFn: getAdminVirtualMemberships,
    enabled: open,
  })
  const memberships = useMemo(() => data?.data ?? [], [data?.data])
  const filtered = useMemo(() => {
    const keyword = query.trim().toLocaleLowerCase()
    return memberships.filter((membership) => {
      if (status !== 'all' && membership.status !== status) return false
      if (!keyword) return true
      return [
        membership.username,
        membership.display_name,
        membership.email,
        membership.plan_title,
        membership.plan_code,
        String(membership.user_id),
        String(membership.id),
      ].some((value) => value?.toLocaleLowerCase().includes(keyword))
    })
  }, [memberships, query, status])
  const activeCount = memberships.filter(
    (membership) => membership.status === 'active'
  ).length
  const userCount = new Set(memberships.map((membership) => membership.user_id))
    .size
  const activeRemaining = memberships
    .filter((membership) => membership.status === 'active')
    .reduce((sum, membership) => sum + membership.weekly_remaining, 0)

  const deleteMembership = async () => {
    if (!deleteTarget || deleting) return
    setDeleting(true)
    try {
      const result = await deleteAdminVirtualMembership(deleteTarget.id)
      if (!result.success) {
        toast.error(result.message || '删除虚拟会员失败')
        return
      }
      const unbound = result.data?.unbound_tokens ?? 0
      toast.success(
        unbound > 0
          ? `虚拟会员已删除，已解除 ${unbound} 个 API Key 的会员绑定`
          : '虚拟会员已删除'
      )
      setDeleteTarget(null)
      await refetch()
    } catch {
      // The shared API interceptor presents backend failures, including a
      // membership that still has an in-flight settlement.
    } finally {
      setDeleting(false)
    }
  }

  const resetMembership = async (membership: AdminVirtualMembership) => {
    if (
      resettingId !== null ||
      !window.confirm(
        `确认重置 ${membership.display_name || membership.username || `用户 #${membership.user_id}`} 的 ${membership.plan_title} 额度吗？`
      )
    )
      return
    setResettingId(membership.id)
    try {
      const result = await resetAdminVirtualMemberships({
        membership_id: membership.id,
      })
      if (!result.success) {
        toast.error(result.message || '重置虚拟会员失败')
        return
      }
      toast.success(`已重置 ${result.data?.affected ?? 0} 个会员实例`)
      await refetch()
    } finally {
      setResettingId(null)
    }
  }

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent
          className={sideDrawerContentClassName(
            'w-[96vw] sm:max-w-[min(1100px,96vw)]'
          )}
        >
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle className='flex items-center gap-2'>
              <Users className='size-4 text-emerald-600' />
              已购虚拟会员
            </SheetTitle>
            <SheetDescription>
              查看购买用户、会员余额、周期重置时间与有效状态
            </SheetDescription>
          </SheetHeader>

          <div className='min-h-0 flex-1 overflow-y-auto p-4 sm:p-6'>
            <div className='grid gap-2 sm:grid-cols-2 xl:grid-cols-4'>
              <SummaryCard
                icon={<WalletCards className='size-4 text-emerald-600' />}
                label='会员实例'
                value={memberships.length}
              />
              <SummaryCard
                icon={<Gauge className='size-4 text-blue-600' />}
                label='生效中'
                value={activeCount}
              />
              <SummaryCard
                icon={<UserRound className='size-4 text-amber-600' />}
                label='购买用户'
                value={userCount}
              />
              <SummaryCard
                icon={<WalletCards className='size-4 text-rose-600' />}
                label='生效会员周余额'
                value={quotaText(activeRemaining)}
              />
            </div>

            <div className='my-4 flex flex-col gap-2 sm:flex-row'>
              <div className='relative min-w-0 flex-1'>
                <Search className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2' />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  className='pl-9'
                  placeholder='搜索用户名、邮箱、用户 ID 或套餐'
                />
              </div>
              <Select
                value={status}
                onValueChange={(value) => setStatus(value as MembershipStatus)}
              >
                <SelectTrigger className='w-full sm:w-32'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>全部状态</SelectItem>
                  <SelectItem value='active'>生效中</SelectItem>
                  <SelectItem value='expired'>已过期</SelectItem>
                  <SelectItem value='cancelled'>已取消</SelectItem>
                </SelectContent>
              </Select>
              <Button onClick={() => setGrantOpen(true)}>
                <UserPlus className='size-4' />
                手动添加会员
              </Button>
              <Button
                variant='outline'
                onClick={() => void refetch()}
                disabled={isFetching}
              >
                <RefreshCw
                  className={cn('size-4', isFetching && 'animate-spin')}
                />
                刷新
              </Button>
            </div>

            <div className='overflow-hidden rounded-xl border'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className='pl-4'>用户</TableHead>
                    <TableHead>会员方案</TableHead>
                    <TableHead>周余额 / 重置</TableHead>
                    <TableHead>5 小时余额 / 重置</TableHead>
                    <TableHead>有效期与限制</TableHead>
                    <TableHead className='pr-4 text-right'>状态</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {isLoading ? (
                    Array.from({ length: 5 }).map((_, index) => (
                      <TableRow key={index}>
                        <TableCell colSpan={6} className='px-4 py-3'>
                          <Skeleton className='h-12 w-full' />
                        </TableCell>
                      </TableRow>
                    ))
                  ) : filtered.length === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={6}
                        className='text-muted-foreground h-32 text-center'
                      >
                        没有符合条件的虚拟会员
                      </TableCell>
                    </TableRow>
                  ) : (
                    filtered.map((membership) => {
                      const meta =
                        statusMeta[
                          membership.status as Exclude<MembershipStatus, 'all'>
                        ]
                      return (
                        <TableRow key={membership.id}>
                          <TableCell className='pl-4 align-top'>
                            <div className='max-w-48'>
                              <p className='truncate font-medium'>
                                {membership.display_name ||
                                  membership.username ||
                                  `用户 #${membership.user_id}`}
                              </p>
                              <p className='text-muted-foreground truncate text-[10px]'>
                                @{membership.username || '-'} · ID{' '}
                                {membership.user_id}
                              </p>
                              {membership.email && (
                                <p className='text-muted-foreground mt-1 truncate text-[10px]'>
                                  {membership.email}
                                </p>
                              )}
                            </div>
                          </TableCell>
                          <TableCell className='align-top'>
                            <p className='font-medium'>
                              {membership.plan_title}
                            </p>
                            <p className='text-muted-foreground text-[10px]'>
                              {membership.group_size === 1
                                ? '单独购买'
                                : `${membership.group_size} 人档`}{' '}
                              · 会员 #{membership.id}
                            </p>
                          </TableCell>
                          <TableCell className='align-top'>
                            <QuotaCell
                              remaining={membership.weekly_remaining}
                              total={membership.weekly_quota}
                              percent={membership.weekly_percent}
                              resetAt={membership.weekly_reset_at}
                            />
                          </TableCell>
                          <TableCell className='align-top'>
                            <QuotaCell
                              remaining={membership.five_hour_remaining}
                              total={membership.five_hour_quota}
                              percent={membership.five_hour_percent}
                              resetAt={membership.five_hour_reset_at}
                              disabled={!membership.five_hour_enabled}
                            />
                          </TableCell>
                          <TableCell className='align-top'>
                            <p className='text-xs'>
                              至 {formatTimestampToDate(membership.end_time)}
                            </p>
                            <p className='text-muted-foreground mt-1 text-[10px]'>
                              并发 {membership.concurrency_limit || '不限'} ·
                              RPM {membership.rpm_limit || '不限'}
                            </p>
                          </TableCell>
                          <TableCell className='pr-4 text-right align-top'>
                            <div className='flex flex-col items-end gap-2'>
                              <Badge
                                variant='outline'
                                className={meta?.className}
                              >
                                {meta?.label || membership.status}
                              </Badge>
                              <Button
                                type='button'
                                variant='ghost'
                                size='sm'
                                className='h-7 px-2 text-xs text-emerald-700 hover:bg-emerald-500/10 hover:text-emerald-700'
                                disabled={resettingId !== null}
                                onClick={() => void resetMembership(membership)}
                              >
                                <RefreshCw
                                  className={cn(
                                    'size-3.5',
                                    resettingId === membership.id &&
                                      'animate-spin'
                                  )}
                                />
                                重置额度
                              </Button>
                              <Button
                                type='button'
                                variant='ghost'
                                size='sm'
                                className='text-destructive hover:bg-destructive/10 hover:text-destructive h-7 px-2 text-xs'
                                onClick={() => setDeleteTarget(membership)}
                              >
                                <Trash2 className='size-3.5' />
                                删除
                              </Button>
                            </div>
                            {membership.user_deleted && (
                              <p className='text-destructive mt-1 text-[10px]'>
                                用户已删除
                              </p>
                            )}
                          </TableCell>
                        </TableRow>
                      )
                    })
                  )}
                </TableBody>
              </Table>
            </div>
            <p className='text-muted-foreground mt-3 text-xs'>
              当前显示 {filtered.length} / {memberships.length}{' '}
              个会员实例；余额与重置时间按最新周期实时刷新。
            </p>
          </div>
        </SheetContent>
      </Sheet>
      <GrantMembershipDialog
        open={grantOpen}
        onOpenChange={setGrantOpen}
        onSuccess={async () => {
          await refetch()
        }}
      />
      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(nextOpen) => {
          if (!nextOpen && !deleting) setDeleteTarget(null)
        }}
        title='删除虚拟会员'
        desc={
          deleteTarget ? (
            <div className='space-y-2 text-sm'>
              <p>
                确定删除
                <span className='text-foreground mx-1 font-medium'>
                  {deleteTarget.display_name ||
                    deleteTarget.username ||
                    `用户 #${deleteTarget.user_id}`}
                </span>
                的
                <span className='text-foreground mx-1 font-medium'>
                  {deleteTarget.plan_title}
                </span>
                吗？
              </p>
              <p>
                会员实例会立即移除，购买订单和已结算用量审计仍保留。关联的 API
                Key 会自动解除会员绑定。
              </p>
              <p className='text-destructive'>
                API Key
                的会员专属分组不会改成钱包分组；未重新绑定会员或修改分组前，请求会因无会员额度而失败，不会扣钱包余额。
              </p>
            </div>
          ) : (
            ''
          )
        }
        confirmText='确认删除'
        destructive
        isLoading={deleting}
        handleConfirm={() => void deleteMembership()}
      />
    </>
  )
}
