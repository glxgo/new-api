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
  ArrowDownToLine,
  CircleDollarSign,
  GripVertical,
  Plus,
  Route,
  ShieldCheck,
  Trash2,
} from 'lucide-react'
import { Reorder } from 'motion/react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { UserSubscription } from '@/features/subscriptions/types'
import type { UserVirtualMembership } from '@/features/virtual-membership/types'
import type { ApiKeyRouteStep } from '../types'
import type { ApiKeyGroupOption } from './api-key-group-combobox'

type EditableRoute = ApiKeyRouteStep & { clientKey: string }

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  groups: ApiKeyGroupOption[]
  subscribedGroups: string[]
  virtualMembershipGroups: string[]
  subscriptions: UserSubscription[]
  memberships: UserVirtualMembership[]
  initialSteps: ApiKeyRouteStep[]
  onSave: (steps: ApiKeyRouteStep[]) => void
}

function sourceForGroup(
  group: string,
  subscribedGroups: Set<string>,
  membershipGroups: Set<string>
): ApiKeyRouteStep['funding_source'] {
  if (membershipGroups.has(group)) return 'virtual_membership'
  if (subscribedGroups.has(group)) return 'subscription'
  return 'wallet'
}

function sourceLabel(source: ApiKeyRouteStep['funding_source']) {
  if (source === 'virtual_membership') return '会员额度'
  if (source === 'subscription') return '订阅套餐'
  return '账户余额'
}

function sourceIcon(source: ApiKeyRouteStep['funding_source']) {
  if (source === 'wallet') return CircleDollarSign
  if (source === 'subscription') return ArrowDownToLine
  return ShieldCheck
}

export function ApiKeyRoutingPolicyDialog({
  open,
  onOpenChange,
  groups,
  subscribedGroups,
  virtualMembershipGroups,
  subscriptions,
  memberships,
  initialSteps,
  onSave,
}: Props) {
  const subscribedSet = useMemo(
    () => new Set(subscribedGroups),
    [subscribedGroups]
  )
  const membershipSet = useMemo(
    () => new Set(virtualMembershipGroups),
    [virtualMembershipGroups]
  )
  const [routes, setRoutes] = useState<EditableRoute[]>(() =>
    initialSteps.map((step, index) => ({
      ...step,
      funding_source:
        step.funding_source ||
        sourceForGroup(step.group, subscribedSet, membershipSet),
      clientKey: `${step.id || 0}-${step.group}-${index}`,
    }))
  )

  const selectedGroups = new Set(routes.map((route) => route.group))
  const available = groups.filter(
    (group) => group.value !== 'auto' && !selectedGroups.has(group.value)
  )

  const addGroup = (group: ApiKeyGroupOption) => {
    const source = sourceForGroup(group.value, subscribedSet, membershipSet)
    setRoutes((current) => [
      ...current,
      {
        id: 0,
        position: current.length + 1,
        group: group.value,
        funding_source: source,
        selection_mode: 'auto',
        source_id: 0,
        clientKey: `${group.value}-${Date.now()}`,
      },
    ])
  }

  const updateRoute = (clientKey: string, patch: Partial<EditableRoute>) => {
    setRoutes((current) =>
      current.map((route) =>
        route.clientKey === clientKey ? { ...route, ...patch } : route
      )
    )
  }

  const save = () => {
    onSave(
      routes.map(({ clientKey: _clientKey, ...route }, index) => ({
        ...route,
        position: index + 1,
      }))
    )
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[90dvh] flex-col overflow-hidden p-0 sm:max-w-3xl'>
        <DialogHeader className='border-b px-5 pt-5 pb-4 sm:px-6'>
          <DialogTitle className='flex items-center gap-2 text-lg'>
            <Route className='text-primary size-5' />
            API Key 消耗路由策略
          </DialogTitle>
          <DialogDescription className='max-w-2xl leading-5'>
            只选择这枚 Key 允许消耗的真实分组，再拖动调整优先级。系统每次都从第
            1 位重新判断：会员或套餐重置后会自动恢复优先；故障分组会暂时跳过。
          </DialogDescription>
        </DialogHeader>

        <div className='min-h-0 flex-1 space-y-6 overflow-y-auto px-5 py-5 sm:px-6'>
          <section className='space-y-3'>
            <div className='flex items-end justify-between gap-3'>
              <div>
                <h3 className='text-sm font-semibold'>消耗顺序</h3>
                <p className='text-muted-foreground mt-1 text-xs'>
                  拖动整张卡片排序。未加入的分组永远不会被这枚 Key 消耗。
                </p>
              </div>
              <Badge variant='outline'>{routes.length} / 16</Badge>
            </div>

            <Reorder.Group
              axis='y'
              values={routes}
              onReorder={setRoutes}
              className='space-y-2'
            >
              {routes.map((route, index) => {
                const group = groups.find((item) => item.value === route.group)
                const exactOptions =
                  route.funding_source === 'subscription'
                    ? subscriptions
                        .filter(
                          (item) =>
                            !item.allowed_group ||
                            item.allowed_group === route.group
                        )
                        .map((item) => ({
                          id: item.id,
                          label: `${item.plan_title || '套餐'} · #${item.id}`,
                        }))
                    : memberships
                        .filter(
                          (item) =>
                            (item.allowed_group?.trim() || '') === route.group
                        )
                        .map((item) => ({
                          id: item.id,
                          label: `${item.plan_title} · #${item.id}`,
                        }))
                return (
                  <Reorder.Item
                    key={route.clientKey}
                    value={route}
                    whileDrag={{
                      scale: 1.012,
                      boxShadow: '0 18px 40px -20px rgba(15, 23, 42, .45)',
                    }}
                    className='bg-card relative grid cursor-grab touch-none gap-3 rounded-xl border p-3 shadow-sm select-none active:cursor-grabbing sm:grid-cols-[auto_auto_minmax(0,1fr)_auto] sm:items-center'
                  >
                    <GripVertical className='text-muted-foreground hidden size-5 sm:block' />
                    <span className='bg-primary/10 text-primary grid size-7 place-items-center rounded-full font-mono text-xs font-bold'>
                      {index + 1}
                    </span>
                    <div className='min-w-0 space-y-2'>
                      <div className='flex flex-wrap items-center gap-2'>
                        <span className='truncate text-sm font-semibold'>
                          {group?.label || route.group}
                        </span>
                        <Badge variant='secondary'>
                          {sourceLabel(route.funding_source)}
                        </Badge>
                        {group?.ratio !== undefined && (
                          <Badge variant='outline'>{group.ratio}x 倍率</Badge>
                        )}
                      </div>
                      {route.funding_source !== 'wallet' && (
                        <div className='grid gap-2 sm:grid-cols-[auto_minmax(0,1fr)] sm:items-center'>
                          <RadioGroup
                            value={route.selection_mode}
                            onValueChange={(value) =>
                              updateRoute(route.clientKey, {
                                selection_mode: value as 'auto' | 'instance',
                                source_id: 0,
                              })
                            }
                            className='flex gap-3'
                          >
                            <label className='flex items-center gap-1.5 text-xs'>
                              <RadioGroupItem value='auto' />
                              自动分配
                            </label>
                            <label className='flex items-center gap-1.5 text-xs'>
                              <RadioGroupItem value='instance' />
                              绑定实例
                            </label>
                          </RadioGroup>
                          {route.selection_mode === 'instance' && (
                            <Select
                              value={
                                route.source_id > 0
                                  ? String(route.source_id)
                                  : undefined
                              }
                              onValueChange={(value) =>
                                value !== null &&
                                updateRoute(route.clientKey, {
                                  source_id: Number(value),
                                })
                              }
                            >
                              <SelectTrigger className='h-8'>
                                <SelectValue placeholder='选择要绑定的实例' />
                              </SelectTrigger>
                              <SelectContent>
                                {exactOptions.map((item) => (
                                  <SelectItem
                                    key={item.id}
                                    value={String(item.id)}
                                  >
                                    {item.label}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          )}
                        </div>
                      )}
                    </div>
                    <Button
                      type='button'
                      size='icon-sm'
                      variant='ghost'
                      className='text-muted-foreground hover:text-destructive absolute top-2 right-2 sm:static'
                      onClick={() =>
                        setRoutes((current) =>
                          current.filter(
                            (item) => item.clientKey !== route.clientKey
                          )
                        )
                      }
                      aria-label={`移除 ${route.group}`}
                    >
                      <Trash2 className='size-4' />
                    </Button>
                  </Reorder.Item>
                )
              })}
            </Reorder.Group>

            {routes.length === 0 && (
              <div className='text-muted-foreground grid place-items-center rounded-xl border border-dashed py-10 text-center text-sm'>
                <Route className='mb-2 size-6 opacity-45' />
                从下方加入至少一个明确分组
              </div>
            )}
          </section>

          <section className='space-y-3'>
            <div>
              <h3 className='text-sm font-semibold'>可加入的分组</h3>
              <p className='text-muted-foreground mt-1 text-xs'>
                余额分组也必须选到具体名称；不存在“默认余额分组”。
              </p>
            </div>
            <div className='grid gap-2 sm:grid-cols-2'>
              {available.map((group) => {
                const source = sourceForGroup(
                  group.value,
                  subscribedSet,
                  membershipSet
                )
                const Icon = sourceIcon(source)
                return (
                  <button
                    key={group.value}
                    type='button'
                    disabled={routes.length >= 16}
                    onClick={() => addGroup(group)}
                    className='border-border bg-card hover:border-primary/40 hover:bg-muted/40 flex items-center gap-3 rounded-xl border p-3 text-left transition-colors disabled:opacity-50'
                  >
                    <span className='bg-muted grid size-9 shrink-0 place-items-center rounded-lg'>
                      <Icon className='size-4' />
                    </span>
                    <span className='min-w-0 flex-1'>
                      <span className='block truncate text-sm font-medium'>
                        {group.label}
                      </span>
                      <span className='text-muted-foreground text-xs'>
                        {sourceLabel(source)}
                        {group.ratio !== undefined
                          ? ` · ${group.ratio}x 倍率`
                          : ''}
                      </span>
                    </span>
                    <Plus className='text-primary size-4' />
                  </button>
                )
              })}
            </div>
          </section>

          <div className='border-primary/15 bg-primary/5 text-muted-foreground rounded-xl border p-3 text-xs leading-5'>
            故障转移以“分组 + 模型 + 接口类型”隔离，不会因为 Plus 分组的故障把
            Pro 也一起停用。首选分组恢复后，新请求会再次从它开始。
          </div>
        </div>

        <DialogFooter className='border-t px-5 py-4 sm:px-6'>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button
            disabled={
              routes.length === 0 ||
              routes.some(
                (route) =>
                  route.selection_mode === 'instance' && route.source_id <= 0
              )
            }
            onClick={save}
          >
            使用这个路由策略
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
