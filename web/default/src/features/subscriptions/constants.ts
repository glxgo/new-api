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
import { type TFunction } from 'i18next'

// ============================================================================
// Duration Unit Options
// ============================================================================

export const DURATION_UNITS = [
  { value: 'year', labelKey: 'years' },
  { value: 'month', labelKey: 'months' },
  { value: 'week', labelKey: 'weeks' },
  { value: 'day', labelKey: 'days' },
  { value: 'hour', labelKey: 'hours' },
  { value: 'custom', labelKey: 'Custom (seconds)' },
] as const

export const RESET_PERIODS = [
  { value: 'never', labelKey: 'No Reset' },
  { value: 'daily', labelKey: 'Daily' },
  { value: 'weekly', labelKey: 'Weekly' },
  { value: 'monthly', labelKey: 'Monthly' },
  { value: 'custom', labelKey: 'Custom (seconds)' },
] as const

export function getDurationUnitOptions(t: TFunction) {
  return DURATION_UNITS.map((u) => ({ value: u.value, label: t(u.labelKey) }))
}

export function getResetPeriodOptions(t: TFunction) {
  return RESET_PERIODS.map((p) => ({ value: p.value, label: t(p.labelKey) }))
}

export const PLAN_VERSIONS = [
  { value: 'starter', labelKey: 'Starter Plan' },
  { value: 'advanced', labelKey: 'Advanced Plan' },
  { value: 'pro', labelKey: 'Pro Plan' },
  { value: 'enterprise', labelKey: 'Enterprise Plan' },
] as const

export function getPlanVersionOptions(t: TFunction) {
  return PLAN_VERSIONS.map((v) => ({ value: v.value, label: t(v.labelKey) }))
}

// plan_version enum value -> i18n labelKey (空值返回 '')
export function versionLabelOf(v?: string | null): string {
  if (!v) return ''
  return PLAN_VERSIONS.find((x) => x.value === v)?.labelKey ?? ''
}

// 套餐版本 → 右上徽章配色(卡片本身统一白底灰边, 只靠徽章区分会员等级)。
// 配色对照会员卡图: 入门版=黄金会员 / 进阶版=铂金会员(香槟金) / 专业版=钻石会员(冰蓝) / 企业版=黑钻会员(深蓝黑)。
// 双层结构(wrapper 外层 border 灰边白底 + inner Card border-0)保留, 但各版本 wrapper/inner 相同, 仅 badge 随版本变色。
// subscription-plans-card(可购套餐, 显示徽章) 与 my-subscriptions-detail(余量卡, 无徽章) 共用。
export const PLAN_VERSION_STYLES: Record<
  string,
  { wrapper: string; inner: string; badge: string }
> = {
  // 入门版 = 黄金会员
  starter: {
    wrapper: 'border border-border bg-card',
    inner: 'border-0',
    badge: 'bg-[#FFD700] text-[#5C4500]',
  },
  // 进阶版 = 铂金会员(香槟金)
  advanced: {
    wrapper: 'border border-border bg-card',
    inner: 'border-0',
    badge: 'bg-[#FFD4A3] text-[#7A4E1C]',
  },
  // 专业版 = 钻石会员(冰蓝)
  pro: {
    wrapper: 'border border-border bg-card',
    inner: 'border-0',
    badge: 'bg-[#B8E0FF] text-[#0C4A8C]',
  },
  // 企业版 = 黑钻会员(深蓝黑)
  enterprise: {
    wrapper: 'border border-border bg-card',
    inner: 'border-0',
    badge: 'bg-[#1A237E] text-white',
  },
}
