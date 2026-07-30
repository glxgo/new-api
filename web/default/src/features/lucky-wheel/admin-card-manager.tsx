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
import {
  ChevronLeft,
  ChevronRight,
  History,
  Search,
  ShieldAlert,
  TicketX,
} from 'lucide-react'
import { toast } from 'sonner'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  getLuckyAdminCards,
  revokeLuckyUserCards,
  type LuckyAdminCardsResult,
} from './api'

const PAGE_SIZE = 10

const statusMeta: Record<
  string,
  {
    label: string
    variant: 'default' | 'secondary' | 'destructive' | 'outline'
  }
> = {
  available: { label: '可使用', variant: 'default' },
  consumed: { label: '已抽奖', variant: 'secondary' },
  expired: { label: '已过期', variant: 'outline' },
  revoked: { label: '已作废', variant: 'destructive' },
  review_required: { label: '待审核', variant: 'destructive' },
}

function formatTime(timestamp: number) {
  if (!timestamp) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(timestamp * 1000))
}

function sourceLabel(sourceType: string) {
  const labels: Record<string, string> = {
    admin_compensation: '管理员补发',
    wallet_topup: '充值获得',
    subscription_purchase: '购买套餐',
    subscription_reset: '套餐周期',
  }
  return labels[sourceType] || sourceType || '未知来源'
}

export function LuckyCardManager({ onChanged }: { onChanged: () => void }) {
  const [userIdInput, setUserIdInput] = useState('')
  const [result, setResult] = useState<LuckyAdminCardsResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [revoking, setRevoking] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [reason, setReason] = useState('管理员手动清空未使用幸运卡')

  const counts = useMemo(
    () =>
      new Map(
        (result?.status_counts || []).map((item) => [item.status, item.count])
      ),
    [result]
  )
  const availableCount = counts.get('available') || 0
  const pageCount = Math.max(1, Math.ceil((result?.total || 0) / PAGE_SIZE))

  async function load(page = 1) {
    const userId = Number(userIdInput)
    if (!Number.isInteger(userId) || userId <= 0) {
      toast.error('请输入正确的用户 ID')
      return
    }
    setLoading(true)
    try {
      const response = await getLuckyAdminCards(userId, page, PAGE_SIZE)
      if (response.success) setResult(response.data)
    } finally {
      setLoading(false)
    }
  }

  async function revokeAvailableCards() {
    if (!result?.user || !reason.trim()) {
      toast.error('请填写操作原因')
      return
    }
    setRevoking(true)
    try {
      const response = await revokeLuckyUserCards({
        user_id: result.user.id,
        reason: reason.trim(),
      })
      if (response.success) {
        toast.success(`已作废 ${response.data.revoked_cards} 张未使用幸运卡`)
        setConfirmOpen(false)
        await load(1)
        onChanged()
      }
    } finally {
      setRevoking(false)
    }
  }

  return (
    <>
      <Card className='overflow-hidden'>
        <CardHeader className='border-b bg-[linear-gradient(110deg,hsl(var(--muted)/.55),transparent_65%)]'>
          <div className='flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between'>
            <div>
              <CardTitle className='flex items-center gap-2'>
                <TicketX className='size-5' />
                用户幸运卡管理
              </CardTitle>
              <p className='text-muted-foreground mt-1 text-sm'>
                先查询用户和卡片明细，再作废未使用卡；已中奖记录始终保留。
              </p>
            </div>
            <Badge variant='outline' className='h-6'>
              仅超级管理员
            </Badge>
          </div>
        </CardHeader>
        <CardContent className='space-y-5 pt-5'>
          <div className='flex flex-col gap-3 sm:flex-row sm:items-end'>
            <div className='min-w-0 flex-1 space-y-2'>
              <Label htmlFor='lucky-card-user-id'>用户 ID</Label>
              <Input
                id='lucky-card-user-id'
                type='number'
                min={1}
                value={userIdInput}
                onChange={(event) => setUserIdInput(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') void load(1)
                }}
                placeholder='例如：5'
              />
            </div>
            <Button
              variant='outline'
              disabled={loading}
              onClick={() => void load(1)}
            >
              <Search />
              {loading ? '查询中' : '查询用户'}
            </Button>
          </div>

          {result?.user && (
            <div className='space-y-5'>
              <div className='bg-muted/20 rounded-2xl border p-4'>
                <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                  <div>
                    <div className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
                      当前查询用户
                    </div>
                    <div className='mt-1 text-lg font-semibold'>
                      #{result.user.id} ·{' '}
                      {result.user.display_name || result.user.username}
                    </div>
                    {result.user.display_name && (
                      <div className='text-muted-foreground text-sm'>
                        用户名：{result.user.username}
                      </div>
                    )}
                  </div>
                  <div className='font-mono text-3xl font-semibold tabular-nums'>
                    {result.total}
                    <span className='text-muted-foreground ml-1 text-sm font-normal'>
                      张历史卡片
                    </span>
                  </div>
                </div>
                <div className='mt-4 grid grid-cols-2 gap-2 lg:grid-cols-4'>
                  {[
                    ['available', '当前可用'],
                    ['consumed', '已经抽奖'],
                    ['expired', '自然过期'],
                    ['revoked', '人工作废'],
                  ].map(([status, label]) => (
                    <div
                      key={status}
                      className='bg-background rounded-xl border px-3 py-2.5'
                    >
                      <div className='font-mono text-xl font-semibold'>
                        {counts.get(status) || 0}
                      </div>
                      <div className='text-muted-foreground text-xs'>
                        {label}
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <div className='overflow-hidden rounded-2xl border'>
                <div className='bg-muted/25 text-muted-foreground grid grid-cols-[minmax(0,1fr)_auto] gap-3 border-b px-4 py-3 text-xs font-medium'>
                  <span>卡片来源与有效期</span>
                  <span>状态</span>
                </div>
                {result.items.length > 0 ? (
                  result.items.map((card) => {
                    const meta = statusMeta[card.status] || {
                      label: card.status,
                      variant: 'outline' as const,
                    }
                    return (
                      <div
                        key={card.id}
                        className='grid grid-cols-[minmax(0,1fr)_auto] gap-3 border-b px-4 py-3 last:border-b-0'
                      >
                        <div className='min-w-0'>
                          <div className='flex flex-wrap items-center gap-x-2 gap-y-1'>
                            <span className='font-mono text-sm font-semibold'>
                              #{card.id}
                            </span>
                            <span className='text-sm'>
                              {sourceLabel(card.source_type)}
                            </span>
                            <span className='text-muted-foreground text-xs'>
                              {card.pool_type === 'subscription'
                                ? '套餐奖池'
                                : '充值奖池'}
                            </span>
                          </div>
                          <div className='text-muted-foreground mt-1 text-xs'>
                            发放 {formatTime(card.issued_at)} · 到期{' '}
                            {formatTime(card.expires_at)}
                          </div>
                          {card.revoke_reason && (
                            <div className='text-destructive mt-1 text-xs'>
                              原因：{card.revoke_reason}
                            </div>
                          )}
                        </div>
                        <Badge variant={meta.variant}>{meta.label}</Badge>
                      </div>
                    )
                  })
                ) : (
                  <div className='text-muted-foreground px-4 py-10 text-center text-sm'>
                    该用户没有幸运卡记录
                  </div>
                )}
              </div>

              {pageCount > 1 && (
                <div className='flex items-center justify-end gap-2'>
                  <Button
                    size='sm'
                    variant='outline'
                    disabled={loading || result.page <= 1}
                    onClick={() => void load(result.page - 1)}
                  >
                    <ChevronLeft />
                    上一页
                  </Button>
                  <span className='text-muted-foreground min-w-16 text-center text-xs'>
                    {result.page} / {pageCount}
                  </span>
                  <Button
                    size='sm'
                    variant='outline'
                    disabled={loading || result.page >= pageCount}
                    onClick={() => void load(result.page + 1)}
                  >
                    下一页
                    <ChevronRight />
                  </Button>
                </div>
              )}

              <div className='border-destructive/25 bg-destructive/5 rounded-2xl border p-4'>
                <div className='flex items-start gap-3'>
                  <ShieldAlert className='text-destructive mt-0.5 size-5 shrink-0' />
                  <div className='min-w-0 flex-1'>
                    <div className='font-medium'>清空未使用幸运卡</div>
                    <p className='text-muted-foreground mt-1 text-sm leading-relaxed'>
                      只会作废当前可用的 {availableCount}{' '}
                      张卡。已经抽奖的卡、奖励和历史记录不会删除。
                    </p>
                    <div className='mt-3 grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]'>
                      <Input
                        value={reason}
                        onChange={(event) => setReason(event.target.value)}
                        placeholder='必填：填写清空原因'
                      />
                      <Button
                        variant='destructive'
                        disabled={availableCount === 0 || !reason.trim()}
                        onClick={() => setConfirmOpen(true)}
                      >
                        <TicketX />
                        清空 {availableCount} 张可用卡
                      </Button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          )}

          {!result && (
            <div className='text-muted-foreground flex min-h-36 flex-col items-center justify-center rounded-2xl border border-dashed text-center'>
              <History className='mb-2 size-6 opacity-50' />
              <div className='text-sm'>输入用户 ID 后查看完整卡片状态</div>
              <div className='mt-1 text-xs'>清空前会显示准确数量并再次确认</div>
            </div>
          )}
        </CardContent>
      </Card>

      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认清空未使用幸运卡？</AlertDialogTitle>
            <AlertDialogDescription>
              将作废用户 #{result?.user?.id} 的 {availableCount}{' '}
              张可用幸运卡。该用户将立即无法使用这些卡，但已经抽奖的卡片、奖励和中奖记录会完整保留。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={revoking}>取消</AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
              disabled={revoking}
              onClick={() => void revokeAvailableCards()}
            >
              {revoking ? '正在清空' : `确认作废 ${availableCount} 张`}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
