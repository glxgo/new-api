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
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  ArrowRight,
  CalendarClock,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  CircleDollarSign,
  CreditCard,
  Gift,
  History,
  Info,
  Sparkles,
  TicketCheck,
} from 'lucide-react'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { SectionPageLayout } from '@/components/layout'
import {
  createLuckyDraw,
  getLuckyCards,
  getLuckyDraws,
  getLuckyRules,
  getLuckyWheelStatus,
} from './api'
import type {
  LuckyCard,
  LuckyDraw,
  LuckyPrize,
  LuckyRuleSet,
  LuckyWheelStatus,
} from './types'
import {
  buildWheelBackground,
  buildWheelSegments,
  chooseAvailableCardId,
  chooseNextAvailableCardId,
  formatPrizeProbability,
  getTargetRotation,
  getWheelLabelRotation,
  PRIZE_NAMES,
} from './wheel-model'

const DRAW_PAGE_SIZE = 10

const sourceNames: Record<string, string> = {
  recharge_threshold: '累计充值',
  subscription_purchase: '购买套餐',
  subscription_reset: '套餐周期重置',
  admin_compensation: '人工补偿',
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

function parsePool(raw?: string): LuckyPrize[] {
  try {
    return JSON.parse(raw || '[]') as LuckyPrize[]
  } catch {
    return []
  }
}

function prizeResultText(draw: LuckyDraw) {
  if (draw.prize_type.startsWith('quota_') && draw.actual_usd_micros > 0) {
    return `$${draw.actual_usd_micros / 1_000_000} 套餐额度`
  }
  return PRIZE_NAMES[draw.prize_type] || draw.prize_type
}

export function LuckyWheel() {
  const navigate = useNavigate()
  const [status, setStatus] = useState<LuckyWheelStatus | null>(null)
  const [cards, setCards] = useState<LuckyCard[]>([])
  const [draws, setDraws] = useState<LuckyDraw[]>([])
  const [drawTotal, setDrawTotal] = useState(0)
  const [drawPage, setDrawPage] = useState(1)
  const [drawsLoading, setDrawsLoading] = useState(false)
  const [rules, setRules] = useState<LuckyRuleSet[]>([])
  const [selectedCardId, setSelectedCardId] = useState('')
  const [loading, setLoading] = useState(true)
  const [drawing, setDrawing] = useState(false)
  const [rotation, setRotation] = useState(0)
  const [pendingResult, setPendingResult] = useState<LuckyDraw | null>(null)
  const [pendingNextCardId, setPendingNextCardId] = useState('')
  const [result, setResult] = useState<LuckyDraw | null>(null)
  const [rulesOpen, setRulesOpen] = useState(false)
  const [rechargeGuideOpen, setRechargeGuideOpen] = useState(false)
  const [subscriptionGuideOpen, setSubscriptionGuideOpen] = useState(false)

  const availableCards = useMemo(
    () =>
      cards
        .filter((card) => card.status === 'available')
        .sort((a, b) => a.expires_at - b.expires_at || a.id - b.id),
    [cards]
  )
  const selectedCard = availableCards.find(
    (card) => String(card.id) === selectedCardId
  )
  const activeRule =
    rules.find((rule) => rule.id === selectedCard?.rule_set_id) || rules[0]
  const visiblePool = useMemo(
    () =>
      selectedCard?.pool_type === 'recharge'
        ? parsePool(activeRule?.recharge_pool)
        : parsePool(activeRule?.subscription_pool),
    [
      activeRule?.recharge_pool,
      activeRule?.subscription_pool,
      selectedCard?.pool_type,
    ]
  )
  const wheelSegments = useMemo(
    () =>
      buildWheelSegments(
        visiblePool,
        selectedCard?.pool_type || 'subscription',
        activeRule?.recharge_bonus_usd_micros || 0
      ),
    [
      activeRule?.recharge_bonus_usd_micros,
      selectedCard?.pool_type,
      visiblePool,
    ]
  )
  const wheelLabelByCode = useMemo(
    () => new Map(wheelSegments.map((prize) => [prize.code, prize.label])),
    [wheelSegments]
  )

  async function refresh(preferredCardId = '', requestedDrawPage = 1) {
    const [statusRes, cardsRes, drawsRes, rulesRes] = await Promise.all([
      getLuckyWheelStatus(),
      getLuckyCards(),
      getLuckyDraws(requestedDrawPage, DRAW_PAGE_SIZE),
      getLuckyRules(),
    ])
    if (statusRes.success) setStatus(statusRes.data)
    if (cardsRes.success) setCards(cardsRes.data.items)
    if (drawsRes.success) {
      setDraws(drawsRes.data.items)
      setDrawTotal(drawsRes.data.total)
      setDrawPage(drawsRes.data.page)
    }
    if (rulesRes.success) setRules(rulesRes.data)
    if (cardsRes.success) {
      setSelectedCardId((current) =>
        chooseAvailableCardId(cardsRes.data.items, preferredCardId || current)
      )
    }
  }

  useEffect(() => {
    const loadTimer = window.setTimeout(() => {
      refresh()
        .catch(() => toast.error('幸运大转盘加载失败，请稍后重试'))
        .finally(() => setLoading(false))
    }, 0)
    return () => window.clearTimeout(loadTimer)
  }, [])

  async function handleDraw() {
    if (drawing || loading || !selectedCard || status?.campaign.draw_paused)
      return
    setDrawing(true)
    setResult(null)
    try {
      const key =
        globalThis.crypto?.randomUUID?.() ||
        `draw-${Date.now()}-${Math.random().toString(16).slice(2)}`
      const response = await createLuckyDraw(selectedCard.id, key)
      if (!response.success) {
        setDrawing(false)
        return
      }
      const draw = response.data
      const prizeIndex = wheelSegments.findIndex(
        (prize) => prize.code === draw.prize_type
      )
      if (prizeIndex < 0) {
        throw new Error('抽奖结果不在当前幸运卡奖池中')
      }
      setPendingResult(draw)
      setPendingNextCardId(
        chooseNextAvailableCardId(availableCards, selectedCardId)
      )
      setRotation((current) =>
        getTargetRotation(current, prizeIndex, wheelSegments.length)
      )
    } catch {
      // The global API handler already shows the concrete server error.
      setDrawing(false)
    }
  }

  async function finishDrawReveal() {
    if (!pendingResult) return
    const revealedDraw = pendingResult
    const nextCardId = pendingNextCardId
    setPendingResult(null)
    setPendingNextCardId('')
    setDrawing(false)
    try {
      await refresh(nextCardId, 1)
    } catch {
      toast.error('奖励已发放，但幸运卡列表刷新失败，请稍后刷新页面')
    }
    setResult(revealedDraw)
  }

  async function changeDrawPage(page: number) {
    if (
      drawsLoading ||
      page < 1 ||
      page > Math.max(1, Math.ceil(drawTotal / DRAW_PAGE_SIZE))
    ) {
      return
    }
    setDrawsLoading(true)
    try {
      const response = await getLuckyDraws(page, DRAW_PAGE_SIZE)
      if (response.success) {
        setDraws(response.data.items)
        setDrawTotal(response.data.total)
        setDrawPage(response.data.page)
      }
    } catch {
      toast.error('抽奖记录加载失败，请稍后重试')
    } finally {
      setDrawsLoading(false)
    }
  }

  const progress = status?.recharge_progress
  const previousThreshold =
    progress && progress.highest_awarded_stage > 0 ? progress.eligible_cents : 0
  const progressPercent = progress
    ? Math.min(
        100,
        Math.max(
          0,
          (previousThreshold / Math.max(progress.next_threshold_cents, 1)) * 100
        )
      )
    : 0
  const subscriptionProgress = status?.subscription_progress
  const nextCardDays =
    subscriptionProgress?.eligible && subscriptionProgress.next_card_at > 0
      ? Math.max(
          0,
          Math.ceil(
            (subscriptionProgress.next_card_at - (status?.server_time ?? 0)) /
              (24 * 60 * 60)
          )
        )
      : null
  const drawPageCount = Math.max(1, Math.ceil(drawTotal / DRAW_PAGE_SIZE))

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>幸运大转盘</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-5'>
          {status?.campaign.draw_paused && (
            <Alert>
              <Info />
              <AlertTitle>抽奖暂时暂停</AlertTitle>
              <AlertDescription>
                已有幸运卡的有效期正在冻结，恢复后会按暂停时长自动顺延。
              </AlertDescription>
            </Alert>
          )}

          <div className='grid gap-5 xl:grid-cols-[minmax(0,1.45fr)_minmax(320px,.55fr)]'>
            <section className='relative overflow-hidden rounded-[28px] border border-[#e66d43]/25 bg-[#fff4e7] px-5 py-8 shadow-[0_24px_80px_-42px_rgba(145,55,20,.55)] sm:px-10'>
              <div className='pointer-events-none absolute inset-0 [background-image:radial-gradient(circle_at_15%_12%,rgba(230,109,67,.2),transparent_23%),radial-gradient(circle_at_86%_80%,rgba(234,157,77,.25),transparent_28%)] opacity-50' />
              <div className='relative mx-auto flex max-w-4xl flex-col items-center'>
                <Badge className='mb-3 border-[#e66d43]/30 bg-white/75 text-[#9b3b20]'>
                  <Sparkles className='mr-1 size-3.5' />
                  每张幸运卡可抽一次
                </Badge>
                <h2 className='font-serif text-3xl font-semibold tracking-tight text-[#512719] sm:text-4xl'>
                  转动好运，赢取专属权益
                </h2>
                <p className='mt-2 text-center text-sm text-[#8b513c]'>
                  结果由服务端安全随机产生，转盘动画只负责揭晓惊喜
                </p>

                <div className='relative mt-8 grid place-items-center'>
                  <div className='absolute top-[-11px] z-20 h-0 w-0 border-x-[14px] border-t-[26px] border-x-transparent border-t-[#7b2413] drop-shadow-sm' />
                  <div
                    className={cn(
                      'relative size-[300px] rounded-full border-[12px] border-[#c43f1d] shadow-[0_24px_44px_-20px_rgba(151,52,20,.8)] transition-transform duration-[4800ms] ease-[cubic-bezier(.08,.72,.08,1)] motion-reduce:duration-700 sm:size-[390px]',
                      drawing && 'brightness-105'
                    )}
                    style={{
                      transform: `rotate(${rotation}deg)`,
                      background: buildWheelBackground(wheelSegments.length),
                    }}
                    onTransitionEnd={(event) => {
                      if (
                        event.currentTarget === event.target &&
                        event.propertyName === 'transform'
                      ) {
                        void finishDrawReveal()
                      }
                    }}
                  >
                    {wheelSegments.map((prize, index) => {
                      const angle = (360 / wheelSegments.length) * index
                      return (
                        <div
                          key={prize.code}
                          className='pointer-events-none absolute inset-0'
                          style={{
                            transform: `rotate(${angle}deg)`,
                          }}
                        >
                          <span
                            className='absolute top-[8%] left-1/2 w-[88px] text-center text-[11px] leading-[1.15] font-semibold text-[#6f2b18] sm:top-[9%] sm:w-[110px] sm:text-xs'
                            style={{
                              transform: `translateX(-50%) rotate(${getWheelLabelRotation(angle, rotation)}deg)`,
                            }}
                          >
                            {prize.label}
                          </span>
                        </div>
                      )
                    })}
                  </div>
                  <button
                    type='button'
                    aria-label={drawing ? '好运正在揭晓' : '点击好运开始抽奖'}
                    title={drawing ? '好运正在揭晓' : '点击中心也可以开始抽奖'}
                    disabled={
                      drawing ||
                      loading ||
                      !selectedCard ||
                      status?.campaign.draw_paused
                    }
                    onClick={handleDraw}
                    className={cn(
                      'absolute top-1/2 left-1/2 z-10 grid size-[96px] -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full border-[7px] border-[#ffd297] bg-[#e65f36] text-white shadow-inner transition-[transform,filter] hover:scale-[1.04] hover:brightness-105 focus-visible:ring-4 focus-visible:ring-[#7b2413]/35 focus-visible:outline-none active:scale-95 disabled:cursor-not-allowed disabled:hover:scale-100 disabled:hover:brightness-100 sm:size-[125px]',
                      drawing && 'animate-pulse'
                    )}
                  >
                    <div className='grid place-items-center gap-0.5'>
                      <Sparkles className='size-7' />
                      <span className='text-[10px] font-semibold tracking-[.22em] sm:text-xs'>
                        {drawing ? '揭晓中' : '好运'}
                      </span>
                    </div>
                  </button>
                </div>

                <div className='mt-8 flex w-full max-w-lg flex-col gap-3 sm:flex-row'>
                  <Select
                    value={selectedCardId}
                    onValueChange={(value) =>
                      value !== null && setSelectedCardId(value)
                    }
                    disabled={availableCards.length === 0 || drawing}
                  >
                    <SelectTrigger className='h-11 flex-1 bg-white/85'>
                      <SelectValue placeholder='选择一张幸运卡' />
                    </SelectTrigger>
                    <SelectContent>
                      {availableCards.map((card) => (
                        <SelectItem key={card.id} value={String(card.id)}>
                          #{card.id} ·{' '}
                          {sourceNames[card.source_type] || card.source_type}·{' '}
                          {formatTime(card.expires_at)} 到期
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Button
                    className='h-11 bg-[#c94f2b] px-9 text-white hover:bg-[#ad3e20]'
                    disabled={
                      drawing ||
                      loading ||
                      !selectedCard ||
                      status?.campaign.draw_paused
                    }
                    onClick={handleDraw}
                  >
                    {drawing ? '好运正在揭晓…' : '立即抽奖'}
                  </Button>
                </div>
                {result && (
                  <div
                    role='status'
                    aria-live='polite'
                    className='mt-4 flex w-full max-w-lg items-center gap-3 rounded-xl border border-[#df744d]/30 bg-white/85 px-4 py-3 text-left shadow-sm'
                  >
                    <div className='grid size-10 shrink-0 place-items-center rounded-full bg-[#fff0df] text-[#cb4f2a]'>
                      <Gift className='size-5' />
                    </div>
                    <div className='min-w-0 flex-1'>
                      <p className='text-sm font-semibold text-[#6f2b18]'>
                        恭喜获得 {prizeResultText(result)}
                      </p>
                      <p className='text-muted-foreground mt-0.5 text-xs'>
                        奖励已到账，下一张幸运卡已自动就位
                      </p>
                    </div>
                  </div>
                )}
                <button
                  type='button'
                  className='mt-4 inline-flex items-center gap-1 text-sm text-[#8b513c] underline-offset-4 hover:underline'
                  aria-expanded={rulesOpen}
                  aria-controls='lucky-wheel-rules'
                  onClick={() => setRulesOpen((open) => !open)}
                >
                  {rulesOpen
                    ? '收起奖池概率与活动规则'
                    : '查看奖池概率与活动规则'}
                  <ChevronDown
                    className={cn(
                      'size-3.5 transition-transform',
                      rulesOpen && 'rotate-180'
                    )}
                  />
                </button>
                {rulesOpen && (
                  <section
                    id='lucky-wheel-rules'
                    aria-labelledby='lucky-wheel-rules-title'
                    className='mt-4 w-full max-w-2xl rounded-2xl border border-[#df744d]/25 bg-white/90 p-4 text-left shadow-sm sm:p-5'
                  >
                    <div className='mb-4'>
                      <h3
                        id='lucky-wheel-rules-title'
                        className='font-serif text-xl font-semibold text-[#6f2b18]'
                      >
                        活动规则与奖池概率
                      </h3>
                      <p className='text-muted-foreground mt-1 text-sm'>
                        当前所选幸运卡使用对应来源的独立奖池，概率不会在前端重新计算。
                      </p>
                    </div>
                    <div className='grid gap-2 sm:grid-cols-2'>
                      {visiblePool.map((prize) => (
                        <div
                          key={prize.code}
                          className='flex items-center justify-between rounded-lg border px-3 py-2.5'
                        >
                          <span className='text-sm'>
                            {wheelLabelByCode.get(prize.code) ||
                              PRIZE_NAMES[prize.code] ||
                              prize.code}
                          </span>
                          <span className='font-mono text-sm'>
                            {formatPrizeProbability(prize.weight)}%
                          </span>
                        </div>
                      ))}
                    </div>
                    <div className='bg-muted/40 text-muted-foreground mt-4 space-y-3 rounded-xl p-4 text-sm leading-relaxed'>
                      <p>
                        <strong className='text-foreground'>
                          套餐来源卡：
                        </strong>
                        奖励会跟随来源套餐的限定分组与剩余有效期；套餐额度奖为一次性额度，不继承周期重置。
                      </p>
                      <p>
                        <strong className='text-foreground'>
                          充值来源卡：
                        </strong>
                        套餐额度奖在显示面额上额外增加 $60，有效期固定 30
                        天；不会抽中套餐双倍卡或全额重置卡。
                      </p>
                      <p>
                        <strong className='text-foreground'>钱包赠金：</strong>
                        永久有效，可用于 API 消费，但不能用于购买订阅套餐。
                      </p>
                    </div>
                    <div className='text-muted-foreground mt-4 flex items-center gap-2 text-xs'>
                      <CalendarClock className='size-4' />
                      活动长期有效，直至平台主动暂停；暂停抽奖期间卡片有效期会冻结。
                    </div>
                  </section>
                )}
              </div>
            </section>

            <aside className='space-y-5'>
              <Card>
                <CardHeader>
                  <CardTitle className='flex items-center gap-2'>
                    <TicketCheck className='text-primary size-5' />
                    我的幸运卡
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className='font-mono text-5xl font-semibold'>
                    {status?.available_cards ?? 0}
                  </div>
                  <p className='text-muted-foreground mt-2 text-sm'>
                    系统会默认选择最早到期的一张
                  </p>
                  <div className='mt-5 space-y-2'>
                    {availableCards.slice(0, 3).map((card) => (
                      <button
                        key={card.id}
                        onClick={() => setSelectedCardId(String(card.id))}
                        className={cn(
                          'flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-sm transition-colors',
                          selectedCardId === String(card.id)
                            ? 'border-primary bg-primary/5'
                            : 'hover:bg-muted/60'
                        )}
                      >
                        <span>
                          #{card.id} · {sourceNames[card.source_type]}
                        </span>
                        <span className='text-muted-foreground text-xs'>
                          {formatTime(card.expires_at)}
                        </span>
                      </button>
                    ))}
                    {!loading && availableCards.length === 0 && (
                      <div className='bg-muted/45 text-muted-foreground rounded-lg px-3 py-7 text-center text-sm'>
                        暂无可用幸运卡
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>

              <Card className='overflow-hidden'>
                <button
                  type='button'
                  className='hover:bg-muted/35 focus-visible:ring-primary w-full text-left transition-colors focus-visible:ring-2 focus-visible:outline-none focus-visible:ring-inset'
                  aria-expanded={rechargeGuideOpen}
                  aria-controls='lucky-recharge-guide'
                  onClick={() => setRechargeGuideOpen((open) => !open)}
                >
                  <CardHeader>
                    <CardTitle className='flex items-center gap-2'>
                      <CircleDollarSign className='text-primary size-5' />
                      距离下一张幸运卡
                      <ChevronDown
                        className={cn(
                          'text-muted-foreground ml-auto size-4 transition-transform',
                          rechargeGuideOpen && 'rotate-180'
                        )}
                      />
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className='flex items-end justify-between'>
                      <span className='font-mono text-2xl font-semibold'>
                        ¥{((progress?.eligible_cents ?? 0) / 100).toFixed(2)}
                      </span>
                      <span className='text-muted-foreground text-sm'>
                        / ¥
                        {(
                          (progress?.next_threshold_cents ?? 5000) / 100
                        ).toFixed(2)}
                      </span>
                    </div>
                    <div className='bg-muted mt-3 h-2 overflow-hidden rounded-full'>
                      <div
                        className='h-full rounded-full bg-[#d9653e] transition-[width] duration-500'
                        style={{ width: `${progressPercent}%` }}
                      />
                    </div>
                    <p className='text-muted-foreground mt-3 text-xs leading-relaxed'>
                      累计充值跨过多个档位时，会一次获得对应的全部幸运卡。
                    </p>
                    <p className='text-primary mt-2 text-xs font-medium'>
                      点击查看充值获取规则
                    </p>
                  </CardContent>
                </button>
                {rechargeGuideOpen && (
                  <div
                    id='lucky-recharge-guide'
                    className='bg-muted/20 border-t px-6 py-5'
                  >
                    <p className='text-sm font-semibold'>累计充值发卡规则</p>
                    <ol className='text-muted-foreground mt-3 list-decimal space-y-2 pl-4 text-xs leading-relaxed'>
                      <li>
                        累计充值达到 ¥50、¥100、¥200、¥400、¥600、¥800 时各获得
                        1 张；之后每增加 ¥200 再获得 1 张。
                      </li>
                      <li>
                        一次充值跨过多个档位，会同时补发已跨过档位的全部幸运卡。
                      </li>
                      <li>
                        仅真实充值到账计入累计进度；赠金、后台加额、兑换码、返佣、活动奖励及购买套餐均不计入。
                      </li>
                      <li>充值获得的幸运卡自发放起 30 天内有效。</li>
                    </ol>
                  </div>
                )}
              </Card>

              <Card className='overflow-hidden'>
                <button
                  type='button'
                  className='hover:bg-muted/35 focus-visible:ring-primary w-full text-left transition-colors focus-visible:ring-2 focus-visible:outline-none focus-visible:ring-inset'
                  aria-expanded={subscriptionGuideOpen}
                  aria-controls='lucky-subscription-guide'
                  onClick={() => setSubscriptionGuideOpen((open) => !open)}
                >
                  <CardHeader>
                    <CardTitle className='flex items-center gap-2'>
                      <CreditCard className='text-primary size-5' />
                      购买套餐获得幸运卡
                      <ChevronDown
                        className={cn(
                          'text-muted-foreground ml-auto size-4 transition-transform',
                          subscriptionGuideOpen && 'rotate-180'
                        )}
                      />
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <p className='text-lg leading-snug font-semibold'>
                      购买月卡、周卡可立即获得 3～15 张
                    </p>
                    <p className='text-muted-foreground mt-2 text-xs'>
                      详情以套餐实际显示为准
                    </p>
                  </CardContent>
                </button>
                {subscriptionGuideOpen && (
                  <div
                    id='lucky-subscription-guide'
                    className='bg-muted/20 border-t px-6 py-5'
                  >
                    <p className='text-muted-foreground text-xs leading-relaxed'>
                      将前往订阅套餐页面。具体赠卡数量、是否按周期继续赠卡，以每张套餐卡片的实际标注为准。
                    </p>
                    <Button
                      className='mt-4 w-full'
                      onClick={() =>
                        void navigate({ to: '/subscription-plans' })
                      }
                    >
                      前往购买订阅套餐
                      <ArrowRight className='size-4' />
                    </Button>
                  </div>
                )}
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className='flex items-center gap-2'>
                    <CalendarClock className='text-primary size-5' />
                    距离下次获得幸运卡
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  {!subscriptionProgress?.subscribed ? (
                    <>
                      <p className='text-2xl font-semibold'>未订阅套餐</p>
                      <p className='text-muted-foreground mt-2 text-xs leading-relaxed'>
                        购买符合条件的月卡或周卡后，可查看套餐周期赠卡时间。
                      </p>
                    </>
                  ) : subscriptionProgress.eligible && nextCardDays !== null ? (
                    <>
                      <p className='font-mono text-2xl font-semibold'>
                        剩余 {nextCardDays} 天
                      </p>
                      <p className='text-muted-foreground mt-2 text-xs leading-relaxed'>
                        预计 {formatTime(subscriptionProgress.next_card_at)}
                        随套餐周期重置发放 1 张
                      </p>
                    </>
                  ) : (
                    <>
                      <p className='text-xl font-semibold'>
                        当前套餐暂无周期赠卡
                      </p>
                      <p className='text-muted-foreground mt-2 text-xs leading-relaxed'>
                        无重置套餐只在首次真实购买时按套餐标注发卡。
                      </p>
                    </>
                  )}
                </CardContent>
              </Card>
            </aside>
          </div>

          <Card>
            <CardHeader className='flex flex-row items-center justify-between'>
              <CardTitle className='flex items-center gap-2'>
                <History className='text-primary size-5' />
                抽奖记录
              </CardTitle>
              <span className='text-muted-foreground text-sm'>
                共 {drawTotal} 条
              </span>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>时间</TableHead>
                    <TableHead>幸运卡</TableHead>
                    <TableHead>奖品</TableHead>
                    <TableHead>状态</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {draws.map((draw) => (
                    <TableRow key={draw.id}>
                      <TableCell className='font-mono text-xs'>
                        {formatTime(draw.awarded_at)}
                      </TableCell>
                      <TableCell>#{draw.card_id}</TableCell>
                      <TableCell className='font-medium'>
                        {prizeResultText(draw)}
                      </TableCell>
                      <TableCell>
                        <Badge variant='secondary'>已发放</Badge>
                      </TableCell>
                    </TableRow>
                  ))}
                  {!loading && draws.length === 0 && (
                    <TableRow>
                      <TableCell
                        colSpan={4}
                        className='text-muted-foreground h-28 text-center'
                      >
                        还没有抽奖记录
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
              {drawTotal > DRAW_PAGE_SIZE && (
                <div className='mt-4 flex items-center justify-end gap-2 border-t pt-4'>
                  <Button
                    variant='outline'
                    size='sm'
                    disabled={drawsLoading || drawPage <= 1}
                    onClick={() => void changeDrawPage(drawPage - 1)}
                  >
                    <ChevronLeft className='size-4' />
                    上一页
                  </Button>
                  <span className='text-muted-foreground min-w-20 text-center text-sm'>
                    {drawPage} / {drawPageCount}
                  </span>
                  <Button
                    variant='outline'
                    size='sm'
                    disabled={drawsLoading || drawPage >= drawPageCount}
                    onClick={() => void changeDrawPage(drawPage + 1)}
                  >
                    下一页
                    <ChevronRight className='size-4' />
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
