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
import {
  CirclePause,
  CirclePlay,
  Dices,
  Gift,
  RotateCcw,
  ShieldCheck,
  TicketCheck,
} from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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
import { LuckyCardManager } from './admin-card-manager'
import { LuckyDrawHistory } from './admin-draw-history'
import {
  compensateLuckyCards,
  getLuckyAdminOverview,
  reverseLuckySource,
  setLuckyDrawPaused,
  setLuckyIssuancePaused,
} from './api'
import type { LuckyRuleSet, LuckyWheelStatus } from './types'

interface Overview {
  campaign: LuckyWheelStatus['campaign']
  active_rule: LuckyRuleSet
  cards: Array<{ status: string; count: number }>
  draws: number
}

export function LuckyWheelAdmin() {
  const [overview, setOverview] = useState<Overview | null>(null)
  const [busy, setBusy] = useState('')
  const [reason, setReason] = useState('')
  const [userId, setUserId] = useState('')
  const [count, setCount] = useState('1')
  const [poolType, setPoolType] = useState<'recharge' | 'subscription'>(
    'recharge'
  )
  const [sourceSubscriptionId, setSourceSubscriptionId] = useState('')
  const [ticket, setTicket] = useState('')
  const [reversalSourceType, setReversalSourceType] = useState<
    'wallet_topup' | 'subscription_order'
  >('wallet_topup')
  const [reversalTradeNo, setReversalTradeNo] = useState('')
  const [reversalReason, setReversalReason] = useState('')

  const load = useCallback(async () => {
    const response = await getLuckyAdminOverview()
    if (response.success) setOverview(response.data)
  }, [])

  useEffect(() => {
    const loadTimer = window.setTimeout(() => void load(), 0)
    return () => window.clearTimeout(loadTimer)
  }, [load])

  const cardCounts = useMemo(
    () =>
      new Map((overview?.cards || []).map((item) => [item.status, item.count])),
    [overview]
  )

  async function toggle(kind: 'issuance' | 'draw', paused: boolean) {
    setBusy(kind)
    try {
      const response =
        kind === 'issuance'
          ? await setLuckyIssuancePaused(paused, reason)
          : await setLuckyDrawPaused(paused, reason)
      if (response.success) {
        toast.success(paused ? '活动入口已暂停' : '活动入口已恢复')
        setReason('')
        await load()
      }
    } finally {
      setBusy('')
    }
  }

  async function compensate() {
    const parsedUserId = Number(userId)
    const parsedCount = Number(count)
    if (!parsedUserId || parsedCount < 1 || !ticket.trim()) {
      toast.error('请填写用户 ID、补卡数量和工单号')
      return
    }
    if (poolType === 'subscription' && !Number(sourceSubscriptionId)) {
      toast.error('套餐来源补卡必须填写来源订阅实例')
      return
    }
    setBusy('compensate')
    try {
      const response = await compensateLuckyCards({
        user_id: parsedUserId,
        count: parsedCount,
        pool_type: poolType,
        source_subscription_id:
          poolType === 'subscription'
            ? Number(sourceSubscriptionId)
            : undefined,
        ticket: ticket.trim(),
      })
      if (response.success) {
        toast.success(`已补发 ${response.data.length} 张幸运卡`)
        setTicket('')
        await load()
      }
    } finally {
      setBusy('')
    }
  }

  async function reverseSource() {
    if (!reversalTradeNo.trim() || !reversalReason.trim()) {
      toast.error('请填写交易号和退款/拒付原因')
      return
    }
    setBusy('reversal')
    try {
      const response = await reverseLuckySource({
        source_type: reversalSourceType,
        trade_no: reversalTradeNo.trim(),
        reason: reversalReason.trim(),
      })
      if (response.success) {
        const result = response.data
        toast.success(
          `已撤销 ${result.revoked_cards} 张未使用卡，${result.review_draws} 笔奖励进入审核`
        )
        setReversalTradeNo('')
        setReversalReason('')
        await load()
      }
    } finally {
      setBusy('')
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>幸运大转盘管理</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-5'>
          <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-4'>
            <Card>
              <CardContent className='pt-5'>
                <TicketCheck className='text-primary size-5' />
                <div className='mt-3 font-mono text-3xl font-semibold'>
                  {cardCounts.get('available') || 0}
                </div>
                <div className='text-muted-foreground text-sm'>可用幸运卡</div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className='pt-5'>
                <Dices className='text-primary size-5' />
                <div className='mt-3 font-mono text-3xl font-semibold'>
                  {overview?.draws || 0}
                </div>
                <div className='text-muted-foreground text-sm'>累计抽奖</div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className='pt-5'>
                <ShieldCheck className='text-primary size-5' />
                <div className='mt-3 flex items-center gap-2 text-lg font-semibold'>
                  发卡
                  <Badge
                    variant={
                      overview?.campaign.issuance_paused
                        ? 'destructive'
                        : 'secondary'
                    }
                  >
                    {overview?.campaign.issuance_paused ? '已暂停' : '运行中'}
                  </Badge>
                </div>
                <div className='text-muted-foreground text-sm'>
                  交易与周期入口
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardContent className='pt-5'>
                <Gift className='text-primary size-5' />
                <div className='mt-3 flex items-center gap-2 text-lg font-semibold'>
                  抽奖
                  <Badge
                    variant={
                      overview?.campaign.draw_paused
                        ? 'destructive'
                        : 'secondary'
                    }
                  >
                    {overview?.campaign.draw_paused ? '已暂停' : '运行中'}
                  </Badge>
                </div>
                <div className='text-muted-foreground text-sm'>
                  用户抽奖入口
                </div>
              </CardContent>
            </Card>
          </div>

          <LuckyDrawHistory />

          <div className='grid gap-5 xl:grid-cols-2'>
            <Card>
              <CardHeader>
                <CardTitle>活动开关</CardTitle>
              </CardHeader>
              <CardContent className='space-y-4'>
                <div className='space-y-2'>
                  <Label htmlFor='lucky-reason'>操作原因</Label>
                  <Input
                    id='lucky-reason'
                    value={reason}
                    onChange={(event) => setReason(event.target.value)}
                    placeholder='用于审计，可选'
                  />
                </div>
                <div className='grid gap-3 sm:grid-cols-2'>
                  <Button
                    variant={
                      overview?.campaign.issuance_paused ? 'default' : 'outline'
                    }
                    disabled={busy !== ''}
                    onClick={() =>
                      toggle('issuance', !overview?.campaign.issuance_paused)
                    }
                  >
                    {overview?.campaign.issuance_paused ? (
                      <CirclePlay />
                    ) : (
                      <CirclePause />
                    )}
                    {overview?.campaign.issuance_paused
                      ? '恢复发卡'
                      : '暂停发卡'}
                  </Button>
                  <Button
                    variant={
                      overview?.campaign.draw_paused ? 'default' : 'outline'
                    }
                    disabled={busy !== ''}
                    onClick={() =>
                      toggle('draw', !overview?.campaign.draw_paused)
                    }
                  >
                    {overview?.campaign.draw_paused ? (
                      <CirclePlay />
                    ) : (
                      <CirclePause />
                    )}
                    {overview?.campaign.draw_paused ? '恢复抽奖' : '暂停抽奖'}
                  </Button>
                </div>
                <p className='text-muted-foreground text-xs leading-relaxed'>
                  暂停抽奖会冻结现有卡片有效期；恢复时系统自动按暂停时长顺延。
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>人工补发幸运卡</CardTitle>
              </CardHeader>
              <CardContent className='grid gap-3 sm:grid-cols-2'>
                <div className='space-y-2'>
                  <Label>用户 ID</Label>
                  <Input
                    type='number'
                    value={userId}
                    onChange={(event) => setUserId(event.target.value)}
                  />
                </div>
                <div className='space-y-2'>
                  <Label>数量</Label>
                  <Input
                    type='number'
                    min={1}
                    max={100}
                    value={count}
                    onChange={(event) => setCount(event.target.value)}
                  />
                </div>
                <div className='space-y-2'>
                  <Label>卡片来源奖池</Label>
                  <Select
                    value={poolType}
                    onValueChange={(value) =>
                      value !== null &&
                      setPoolType(value as 'recharge' | 'subscription')
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='recharge'>充值来源</SelectItem>
                      <SelectItem value='subscription'>套餐来源</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-2'>
                  <Label>来源订阅实例</Label>
                  <Input
                    type='number'
                    disabled={poolType !== 'subscription'}
                    value={sourceSubscriptionId}
                    onChange={(event) =>
                      setSourceSubscriptionId(event.target.value)
                    }
                    placeholder='套餐来源时必填'
                  />
                </div>
                <div className='space-y-2 sm:col-span-2'>
                  <Label>工单号 / 补偿依据</Label>
                  <Input
                    value={ticket}
                    onChange={(event) => setTicket(event.target.value)}
                    placeholder='必须唯一，用于防止重复补发'
                  />
                </div>
                <Button
                  className='sm:col-span-2'
                  disabled={busy !== ''}
                  onClick={compensate}
                >
                  确认补发
                </Button>
              </CardContent>
            </Card>
          </div>

          <LuckyCardManager onChanged={() => void load()} />

          <Card>
            <CardHeader>
              <CardTitle>当前规则版本</CardTitle>
            </CardHeader>
            <CardContent className='grid gap-3 text-sm sm:grid-cols-3'>
              <div className='bg-muted/40 rounded-lg p-3'>
                <div className='text-muted-foreground'>版本</div>
                <div className='mt-1 font-mono font-semibold'>
                  v{overview?.active_rule.version || '—'}
                </div>
              </div>
              <div className='bg-muted/40 rounded-lg p-3'>
                <div className='text-muted-foreground'>活动专用分组</div>
                <div className='mt-1 font-semibold'>
                  {overview?.active_rule.activity_group || '—'}
                </div>
              </div>
              <div className='bg-muted/40 rounded-lg p-3'>
                <div className='text-muted-foreground'>充值额度附加</div>
                <div className='mt-1 font-semibold'>
                  +$
                  {(overview?.active_rule.recharge_bonus_usd_micros || 0) /
                    1_000_000}
                </div>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2'>
                <RotateCcw className='size-5' />
                登记退款 / 拒付
              </CardTitle>
            </CardHeader>
            <CardContent className='grid gap-3 sm:grid-cols-2'>
              <div className='space-y-2'>
                <Label>来源类型</Label>
                <Select
                  value={reversalSourceType}
                  onValueChange={(value) =>
                    value !== null &&
                    setReversalSourceType(
                      value as 'wallet_topup' | 'subscription_order'
                    )
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='wallet_topup'>钱包充值订单</SelectItem>
                    <SelectItem value='subscription_order'>
                      订阅购买订单
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className='space-y-2'>
                <Label>交易号</Label>
                <Input
                  value={reversalTradeNo}
                  onChange={(event) => setReversalTradeNo(event.target.value)}
                  placeholder='支付或订阅订单 trade_no'
                />
              </div>
              <div className='space-y-2 sm:col-span-2'>
                <Label>退款 / 拒付原因</Label>
                <Input
                  value={reversalReason}
                  onChange={(event) => setReversalReason(event.target.value)}
                  placeholder='必填，将写入卡片与管理员审计'
                />
              </div>
              <p className='text-muted-foreground text-xs leading-relaxed sm:col-span-2'>
                未使用幸运卡立即撤销；已抽奖的卡片和奖励只进入人工审核，不自动扣除用户已消费权益。
              </p>
              <Button
                className='sm:col-span-2'
                variant='destructive'
                disabled={busy !== ''}
                onClick={reverseSource}
              >
                确认登记并冲销活动资格
              </Button>
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
