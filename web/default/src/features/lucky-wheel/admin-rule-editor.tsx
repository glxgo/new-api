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
import { History, Plus, Save, Send, ShieldCheck, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { activateLuckyRuleSet, createLuckyRuleSet } from './api'
import type { LuckyCard, LuckyPrize, LuckyRuleSet } from './types'
import { formatPrizeProbability, PRIZE_NAMES } from './wheel-model'

const WEIGHT_SCALE = 1_000_000

const prizeCodes = [
  'quota_5',
  'quota_10',
  'quota_20',
  'quota_30',
  'quota_50',
  'quota_100',
  'gift_5',
  'gift_10',
  'gift_20',
  'subscription_double',
  'subscription_full_reset',
  'crazy_5h',
] as const

function parsePool(raw: string) {
  try {
    const value = JSON.parse(raw) as LuckyPrize[]
    return Array.isArray(value) ? value : []
  } catch {
    return []
  }
}

function defaultDisplayAmount(code: string) {
  const match = code.match(/_(\d+)$/)
  if (!match || code === 'crazy_5h') return 0
  return Number(match[1]) * 1_000_000
}

function formatUsdMicros(micros: number) {
  return (micros / 1_000_000).toLocaleString('en-US', {
    maximumFractionDigits: 2,
  })
}

function prizeDisplayName(prize: LuckyPrize) {
  if (prize.code.startsWith('quota_')) {
    return `$${formatUsdMicros(prize.display_usd_micros)} 套餐额度`
  }
  if (prize.code.startsWith('gift_')) {
    return `$${formatUsdMicros(prize.display_usd_micros)} 钱包赠金`
  }
  return PRIZE_NAMES[prize.code] || prize.code
}

function isFixedReward(code: string) {
  return (
    code === 'subscription_double' ||
    code === 'subscription_full_reset' ||
    code === 'crazy_5h'
  )
}

function statusLabel(status: LuckyRuleSet['status']) {
  if (status === 'active') return '生效中'
  if (status === 'draft') return '草稿'
  return '已退役'
}

interface LuckyRuleEditorProps {
  activeRule: LuckyRuleSet | null
  rules: LuckyRuleSet[]
  onChanged: () => Promise<void> | void
}

export function LuckyRuleEditor({
  activeRule,
  rules,
  onChanged,
}: LuckyRuleEditorProps) {
  const [poolType, setPoolType] = useState<LuckyCard['pool_type']>('recharge')
  const [subscriptionPool, setSubscriptionPool] = useState<LuckyPrize[]>(() =>
    parsePool(activeRule?.subscription_pool || '[]')
  )
  const [rechargePool, setRechargePool] = useState<LuckyPrize[]>(() =>
    parsePool(activeRule?.recharge_pool || '[]')
  )
  const [newPrizeCode, setNewPrizeCode] = useState('')
  const [busy, setBusy] = useState('')

  const pool = poolType === 'recharge' ? rechargePool : subscriptionPool
  const setPool =
    poolType === 'recharge' ? setRechargePool : setSubscriptionPool
  const totalWeight = pool.reduce((sum, prize) => sum + prize.weight, 0)
  const allTotalsValid =
    subscriptionPool.reduce((sum, prize) => sum + prize.weight, 0) ===
      WEIGHT_SCALE &&
    rechargePool.reduce((sum, prize) => sum + prize.weight, 0) === WEIGHT_SCALE
  const availablePrizeCodes = useMemo(
    () =>
      prizeCodes.filter(
        (code) =>
          !pool.some((prize) => prize.code === code) &&
          (poolType === 'subscription' ||
            (code !== 'subscription_double' &&
              code !== 'subscription_full_reset'))
      ),
    [pool, poolType]
  )

  function updatePrize(index: number, patch: Partial<LuckyPrize>) {
    setPool(
      pool.map((prize, row) => (row === index ? { ...prize, ...patch } : prize))
    )
  }

  function addPrize() {
    if (!newPrizeCode) return
    setPool([
      ...pool,
      {
        code: newPrizeCode,
        display_usd_micros: defaultDisplayAmount(newPrizeCode),
        weight: 1_000,
      },
    ])
    setNewPrizeCode('')
  }

  async function saveRule(publish: boolean) {
    if (!activeRule || !allTotalsValid) {
      toast.error('两个奖池的概率合计都必须精确等于 100%')
      return
    }
    if (subscriptionPool.length === 0 || rechargePool.length === 0) {
      toast.error('奖池不能为空')
      return
    }
    setBusy(publish ? 'publish' : 'save')
    try {
      const created = await createLuckyRuleSet({
        base_rule_set_id: activeRule.id,
        subscription_pool: JSON.stringify(subscriptionPool),
        recharge_pool: JSON.stringify(rechargePool),
      })
      if (!created.success) return
      if (publish) {
        const activated = await activateLuckyRuleSet(created.data.id)
        if (!activated.success) return
        toast.success(
          `奖池 V${activated.data.version} 已发布，新发幸运卡立即生效`
        )
      } else {
        toast.success(`已保存奖池草稿 V${created.data.version}`)
      }
      await onChanged()
    } finally {
      setBusy('')
    }
  }

  async function activateDraft(rule: LuckyRuleSet) {
    setBusy(`activate-${rule.id}`)
    try {
      const response = await activateLuckyRuleSet(rule.id)
      if (response.success) {
        toast.success(`奖池 V${response.data.version} 已发布`)
        await onChanged()
      }
    } finally {
      setBusy('')
    }
  }

  return (
    <Card className='overflow-hidden'>
      <CardHeader className='border-b bg-[linear-gradient(115deg,rgba(217,101,62,.1),transparent_58%)]'>
        <div className='flex flex-wrap items-start justify-between gap-3'>
          <div>
            <CardTitle className='flex items-center gap-2'>
              <ShieldCheck className='text-primary size-5' />
              奖池与概率管理
            </CardTitle>
            <p className='text-muted-foreground mt-2 max-w-3xl text-sm leading-relaxed'>
              发布会创建一个不可变的新版本并保留旧版本审计记录；所有尚未使用的幸运卡会立即按最新版本抽奖。
            </p>
          </div>
          <Badge variant='secondary'>当前 V{activeRule?.version || '—'}</Badge>
        </div>
      </CardHeader>
      <CardContent className='space-y-5 pt-5'>
        <div className='bg-muted/35 grid grid-cols-2 gap-1 rounded-xl border p-1'>
          {(['recharge', 'subscription'] as const).map((type) => (
            <button
              key={type}
              type='button'
              className={cn(
                'rounded-lg px-3 py-2.5 text-sm font-semibold transition-colors',
                poolType === type
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              )}
              onClick={() => setPoolType(type)}
            >
              {type === 'recharge' ? '充值来源奖池' : '套餐来源奖池'}
            </button>
          ))}
        </div>

        <Alert
          variant={totalWeight === WEIGHT_SCALE ? 'default' : 'destructive'}
        >
          <ShieldCheck />
          <AlertTitle>
            当前奖池合计 {formatPrizeProbability(totalWeight)}%
          </AlertTitle>
          <AlertDescription>
            后端使用 1,000,000 份整数权重；只有精确合计 100% 才允许保存或发布。
            这里的金额同时决定用户实际获得的额度，充值来源奖池抽到什么就发放什么。
          </AlertDescription>
        </Alert>

        <div className='overflow-x-auto rounded-xl border'>
          <div className='min-w-[720px]'>
            <div className='text-muted-foreground bg-muted/35 grid grid-cols-[minmax(180px,1.2fr)_minmax(150px,.8fr)_minmax(160px,.8fr)_48px] gap-3 border-b px-4 py-2 text-xs font-semibold'>
              <span>奖项</span>
              <span>发放金额 / 公示金额（美元）</span>
              <span>概率（%）</span>
              <span />
            </div>
            {pool.map((prize, index) => (
              <div
                key={prize.code}
                className='grid grid-cols-[minmax(180px,1.2fr)_minmax(150px,.8fr)_minmax(160px,.8fr)_48px] items-center gap-3 border-b px-4 py-3 last:border-b-0'
              >
                <div>
                  <div className='font-medium'>{prizeDisplayName(prize)}</div>
                  <code className='text-muted-foreground text-xs'>
                    {prize.code}
                  </code>
                </div>
                <Input
                  type='number'
                  min={0}
                  step='0.01'
                  disabled={isFixedReward(prize.code)}
                  value={prize.display_usd_micros / 1_000_000}
                  onChange={(event) =>
                    updatePrize(index, {
                      display_usd_micros: Math.round(
                        Math.max(0, Number(event.target.value) || 0) * 1_000_000
                      ),
                    })
                  }
                />
                <Input
                  type='number'
                  min={0.0001}
                  max={100}
                  step='0.0001'
                  value={prize.weight / 10_000}
                  onChange={(event) =>
                    updatePrize(index, {
                      weight: Math.round(
                        Math.max(0, Number(event.target.value) || 0) * 10_000
                      ),
                    })
                  }
                />
                <Button
                  type='button'
                  size='icon'
                  variant='ghost'
                  aria-label={`删除 ${prizeDisplayName(prize)}`}
                  onClick={() =>
                    setPool(pool.filter((_, row) => row !== index))
                  }
                >
                  <Trash2 className='size-4' />
                </Button>
              </div>
            ))}
          </div>
        </div>

        <div className='grid gap-3 rounded-xl border border-dashed p-4 sm:grid-cols-[1fr_auto]'>
          <div className='space-y-2'>
            <Label>添加奖项</Label>
            <Select
              value={newPrizeCode}
              onValueChange={(value) => setNewPrizeCode(value || '')}
            >
              <SelectTrigger>
                <SelectValue placeholder='选择当前奖池中尚未包含的奖项' />
              </SelectTrigger>
              <SelectContent>
                {availablePrizeCodes.map((code) => (
                  <SelectItem key={code} value={code}>
                    {PRIZE_NAMES[code] || code}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <Button
            className='self-end'
            variant='outline'
            disabled={!newPrizeCode}
            onClick={addPrize}
          >
            <Plus className='size-4' />
            添加
          </Button>
        </div>

        <div className='flex flex-wrap justify-end gap-3 border-t pt-5'>
          <Button
            variant='outline'
            disabled={busy !== '' || !allTotalsValid}
            onClick={() => void saveRule(false)}
          >
            <Save className='size-4' />
            {busy === 'save' ? '保存中…' : '保存为草稿'}
          </Button>
          <Button
            disabled={busy !== '' || !allTotalsValid}
            onClick={() => void saveRule(true)}
          >
            <Send className='size-4' />
            {busy === 'publish' ? '发布中…' : '发布新版本'}
          </Button>
        </div>

        <div className='border-t pt-5'>
          <div className='mb-3 flex items-center gap-2 font-semibold'>
            <History className='size-4' />
            版本历史
          </div>
          <div className='space-y-2'>
            {rules.map((rule) => (
              <div
                key={rule.id}
                className='flex flex-wrap items-center gap-3 rounded-lg border px-3 py-2 text-sm'
              >
                <span className='font-mono font-semibold'>V{rule.version}</span>
                <Badge
                  variant={rule.status === 'active' ? 'default' : 'secondary'}
                >
                  {statusLabel(rule.status)}
                </Badge>
                <span className='text-muted-foreground'>
                  基于规则 #{rule.base_rule_set_id || '初始'}
                </span>
                <code className='text-muted-foreground ml-auto text-xs'>
                  {rule.checksum?.slice(0, 12) || '—'}
                </code>
                {rule.status === 'draft' && (
                  <Button
                    size='sm'
                    variant='outline'
                    disabled={busy !== ''}
                    onClick={() => void activateDraft(rule)}
                  >
                    {busy === `activate-${rule.id}` ? '激活中…' : '激活草稿'}
                  </Button>
                )}
              </div>
            ))}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
