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
import { RotateCcw, Save, Sparkles, Users } from 'lucide-react'
import { toast } from 'sonner'
import {
  formatQuota,
  parseQuotaFromDollars,
  quotaUnitsToDollars,
} from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { SectionPageLayout } from '@/components/layout'
import { getGroups } from '@/features/subscriptions/api'
import {
  getAdminVirtualMembershipPlans,
  getAdminVirtualMembershipSetting,
  resetAdminVirtualMemberships,
  saveAdminVirtualMembershipPlan,
  saveAdminVirtualMembershipSetting,
} from '@/features/virtual-membership/api'
import type { VirtualMembershipPlan } from '@/features/virtual-membership/types'
import { AdminMembershipsSheet } from './components/admin-memberships-sheet'

const emptyPlan: Partial<VirtualMembershipPlan> = {
  code: '',
  title: '',
  subtitle: '',
  description: '',
  original_price_amount: 0,
  price_amount: 0,
  two_group_original_price: 0,
  two_group_price: 0,
  three_group_original_price: 0,
  three_group_price: 0,
  four_group_original_price: 0,
  four_group_price: 0,
  fixed_profit_amount: 0,
  currency: 'USD',
  duration_days: 30,
  weekly_quota: 0,
  five_hour_enabled: false,
  five_hour_quota: 0,
  concurrency_limit: 0,
  rpm_limit: 0,
  allowed_models: '',
  allowed_group: 'gpt会员分组',
  recommended: false,
  enabled: true,
  sort_order: 0,
}

function planToEditor(plan: VirtualMembershipPlan) {
  return {
    ...plan,
    weekly_quota: quotaUnitsToDollars(plan.weekly_quota),
    five_hour_quota: quotaUnitsToDollars(plan.five_hour_quota),
  }
}

export function VirtualMemberships() {
  const [plans, setPlans] = useState<VirtualMembershipPlan[]>([])
  const [setting, setSetting] = useState({ announcement: '', enabled: true })
  const [editing, setEditing] = useState<Partial<VirtualMembershipPlan>>({
    ...emptyPlan,
  })
  const [groupOptions, setGroupOptions] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [membershipsOpen, setMembershipsOpen] = useState(false)
  const [resetPlanCode, setResetPlanCode] = useState('all')

  const load = async () => {
    setLoading(true)
    try {
      const [planResult, settingResult, groupsResult] =
        await Promise.allSettled([
          getAdminVirtualMembershipPlans(),
          getAdminVirtualMembershipSetting(),
          getGroups(),
        ])
      if (planResult.status === 'fulfilled')
        setPlans(planResult.value.data ?? [])
      if (settingResult.status === 'fulfilled' && settingResult.value.data) {
        setSetting(settingResult.value.data)
      }
      if (groupsResult.status === 'fulfilled' && groupsResult.value.success) {
        setGroupOptions(groupsResult.value.data ?? [])
      }
    } finally {
      setLoading(false)
    }
  }
  // The loader hydrates server state on mount; its state updates are intentional.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load()
  }, [])

  const updateField = (field: string, value: string | number | boolean) =>
    setEditing((current) => ({ ...current, [field]: value }))
  const savePlan = async () => {
    if (
      editing.five_hour_enabled &&
      Number(editing.five_hour_quota ?? 0) <= 0
    ) {
      toast.error('开启 5 小时限额后，请填写大于 0 的额度')
      return
    }
    try {
      const result = await saveAdminVirtualMembershipPlan({
        ...editing,
        weekly_quota: parseQuotaFromDollars(Number(editing.weekly_quota ?? 0)),
        five_hour_quota: parseQuotaFromDollars(
          Number(editing.five_hour_quota ?? 0)
        ),
      })
      if (result.success) {
        toast.success('方案已保存')
        setEditing({ ...emptyPlan })
        await load()
      }
    } catch {
      /* interceptor handles error */
    }
  }
  const saveAnnouncement = async () => {
    const result = await saveAdminVirtualMembershipSetting(setting)
    if (result.success) toast.success('公告已保存')
  }
  const resetByPlan = async () => {
    const target =
      resetPlanCode === 'all'
        ? '所有虚拟会员'
        : `${plans.find((plan) => plan.code === resetPlanCode)?.title ?? resetPlanCode} 用户`
    if (!window.confirm(`确认重置${target}的周额度和 5 小时额度吗？`)) return
    const result = await resetAdminVirtualMemberships(
      resetPlanCode === 'all' ? {} : { plan_code: resetPlanCode }
    )
    if (result.success)
      toast.success(`已重置 ${result.data?.affected ?? 0} 个虚拟会员`)
  }
  const editingGroup = editing.allowed_group?.trim() ?? ''
  const groups = Array.from(
    new Set(groupOptions.concat(editingGroup ? [editingGroup] : []))
  ).sort()

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>虚拟会员管理</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <div className='flex flex-wrap items-center gap-2'>
            <Button
              variant='outline'
              size='sm'
              onClick={() => setMembershipsOpen(true)}
            >
              <Users className='size-4' />
              已购会员
            </Button>
            <Select
              value={resetPlanCode}
              onValueChange={(value) => setResetPlanCode(value ?? 'all')}
            >
              <SelectTrigger className='h-8 w-40'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='all'>全部会员类型</SelectItem>
                {plans.map((plan) => (
                  <SelectItem key={plan.id} value={plan.code}>
                    {plan.title}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button variant='outline' size='sm' onClick={resetByPlan}>
              <RotateCcw className='size-4' />
              按类型重置
            </Button>
          </div>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='grid gap-5 xl:grid-cols-[minmax(0,1fr)_22rem]'>
            <div className='space-y-4'>
              <div className='bg-card rounded-2xl border p-5'>
                <div className='flex items-center gap-2'>
                  <Sparkles className='size-5 text-emerald-600' />
                  <h2 className='font-semibold'>顶部公告</h2>
                </div>
                <Textarea
                  className='mt-4 min-h-28'
                  value={setting.announcement}
                  onChange={(event) =>
                    setSetting({ ...setting, announcement: event.target.value })
                  }
                  placeholder='支持 Markdown，展示在虚拟会员页面顶部'
                />
                <div className='mt-3 flex items-center justify-between'>
                  <label className='text-sm'>
                    <input
                      className='mr-2'
                      type='checkbox'
                      checked={setting.enabled}
                      onChange={(event) =>
                        setSetting({
                          ...setting,
                          enabled: event.target.checked,
                        })
                      }
                    />
                    开启虚拟会员
                  </label>
                  <Button size='sm' onClick={saveAnnouncement}>
                    <Save className='size-4' />
                    保存公告
                  </Button>
                </div>
              </div>
              <div className='bg-card overflow-hidden rounded-2xl border'>
                <div className='border-b p-5'>
                  <h2 className='font-semibold'>方案列表</h2>
                  <p className='text-muted-foreground mt-1 text-xs'>
                    周额度和 5 小时额度按购买档位自动除以 2/3/4。
                  </p>
                </div>
                {loading ? (
                  <div className='text-muted-foreground p-8 text-center'>
                    加载中…
                  </div>
                ) : (
                  plans.map((plan) => (
                    <button
                      key={plan.id}
                      type='button'
                      onClick={() => setEditing(planToEditor(plan))}
                      className='hover:bg-muted/50 flex w-full items-center justify-between border-b p-5 text-left last:border-b-0'
                    >
                      <div>
                        <p className='font-medium'>
                          {plan.title}{' '}
                          <span className='text-muted-foreground ml-2 text-xs'>
                            {plan.code}
                          </span>
                        </p>
                        <p className='text-muted-foreground mt-1 text-xs'>
                          周 {formatQuota(plan.weekly_quota)} · 5h{' '}
                          {plan.five_hour_enabled
                            ? formatQuota(plan.five_hour_quota)
                            : '关闭'}
                          {' · 并发 '}
                          {plan.concurrency_limit > 0
                            ? plan.concurrency_limit
                            : '不限'}
                          {' · RPM '}
                          {plan.rpm_limit > 0 ? plan.rpm_limit : '不限'}
                          {' · 固定利润 $'}
                          {plan.fixed_profit_amount}
                        </p>
                      </div>
                      <span className='text-emerald-600'>编辑</span>
                    </button>
                  ))
                )}
              </div>
            </div>
            <div className='bg-card rounded-2xl border p-5'>
              <h2 className='font-semibold'>
                {editing.id ? '编辑方案' : '新增方案'}
              </h2>
              <div className='mt-4 space-y-3'>
                <Input
                  placeholder='编码，例如 plus'
                  value={editing.code ?? ''}
                  onChange={(event) => updateField('code', event.target.value)}
                />
                <Input
                  placeholder='名称，例如 GPT Plus'
                  value={editing.title ?? ''}
                  onChange={(event) => updateField('title', event.target.value)}
                />
                <Input
                  placeholder='副标题'
                  value={editing.subtitle ?? ''}
                  onChange={(event) =>
                    updateField('subtitle', event.target.value)
                  }
                />
                <Textarea
                  placeholder='方案说明'
                  value={editing.description ?? ''}
                  onChange={(event) =>
                    updateField('description', event.target.value)
                  }
                />
                <div className='grid grid-cols-2 gap-2'>
                  {[
                    ['original_price_amount', '单独原价'],
                    ['price_amount', '单独现价'],
                    ['two_group_original_price', '2 人团原价'],
                    ['two_group_price', '2 人团现价'],
                    ['three_group_original_price', '3 人团原价'],
                    ['three_group_price', '3 人团现价'],
                    ['four_group_original_price', '4 人团原价'],
                    ['four_group_price', '4 人团现价'],
                    ['fixed_profit_amount', '单账号固定利润（美元）'],
                    ['weekly_quota', '周额度（美元）'],
                    ['duration_days', '有效天数'],
                    ['concurrency_limit', '会员并发上限'],
                    ['rpm_limit', '会员 RPM 上限'],
                  ].map(([field, label]) => (
                    <label key={field} className='text-xs'>
                      <span className='text-muted-foreground mb-1 block'>
                        {label}
                      </span>
                      <Input
                        type='number'
                        value={Number(
                          editing[field as keyof typeof editing] ?? 0
                        )}
                        onChange={(event) =>
                          updateField(field, Number(event.target.value))
                        }
                        min={0}
                      />
                    </label>
                  ))}
                </div>
                <Input
                  placeholder='允许模型，逗号分隔；留空表示不限'
                  value={editing.allowed_models ?? ''}
                  onChange={(event) =>
                    updateField('allowed_models', event.target.value)
                  }
                />
                <label className='block text-xs'>
                  <span className='text-muted-foreground mb-1 block'>
                    限定分组
                  </span>
                  <Select
                    value={editingGroup || '__none__'}
                    onValueChange={(value) =>
                      updateField(
                        'allowed_group',
                        value === '__none__' ? '' : (value ?? '')
                      )
                    }
                  >
                    <SelectTrigger>
                      <SelectValue placeholder='不限分组' />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='__none__'>不限分组</SelectItem>
                      {groups.map((group) => (
                        <SelectItem key={group} value={group}>
                          {group}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <span className='text-muted-foreground mt-1 block'>
                    与订阅套餐使用同一组列表；选择“不限分组”表示不限制。
                  </span>
                </label>
                <label className='block text-sm'>
                  <input
                    className='mr-2'
                    type='checkbox'
                    checked={editing.five_hour_enabled ?? false}
                    onChange={(event) =>
                      updateField('five_hour_enabled', event.target.checked)
                    }
                  />
                  开启 5 小时限额
                </label>
                <label className='block text-xs'>
                  <span className='text-muted-foreground mb-1 block'>
                    5 小时额度（美元，开启后生效）
                  </span>
                  <Input
                    type='number'
                    min={0}
                    value={editing.five_hour_quota ?? ''}
                    onChange={(event) =>
                      updateField(
                        'five_hour_quota',
                        event.target.value === ''
                          ? 0
                          : Number(event.target.value)
                      )
                    }
                    placeholder='例如 20'
                  />
                  <span className='text-muted-foreground mt-1 block'>
                    保存时会换算为系统额度；开启后按 2/3/4
                    人档位自动均分，并同步到已生效会员。
                  </span>
                </label>
                <p className='text-muted-foreground text-xs'>
                  并发和 RPM 填 0 表示不限；2/3/4 人档位会按人数向下均分，最小为
                  1。
                </p>
                <label className='block text-sm'>
                  <input
                    className='mr-2'
                    type='checkbox'
                    checked={editing.recommended ?? false}
                    onChange={(event) =>
                      updateField('recommended', event.target.checked)
                    }
                  />
                  推荐方案
                </label>
                <div className='flex gap-2 pt-2'>
                  <Button className='flex-1' onClick={savePlan}>
                    <Save className='size-4' />
                    保存方案
                  </Button>
                  <Button
                    variant='outline'
                    onClick={() => setEditing({ ...emptyPlan })}
                  >
                    清空
                  </Button>
                </div>
              </div>
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>
      <AdminMembershipsSheet
        open={membershipsOpen}
        onOpenChange={setMembershipsOpen}
      />
    </>
  )
}
