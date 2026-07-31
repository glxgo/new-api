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

export interface PlanVersionStyle {
  wrapper: string
  inner: string
  badge: string
  accent: string
  ring: string
}

// 套餐版本配色由可购套餐与余额卡片共用。卡片用轻量渐变区分等级，
// SpecularCard 只负责鼠标高光，不改变业务信息或点击区域。
export const PLAN_VERSION_STYLES: Record<string, PlanVersionStyle> = {
  starter: {
    wrapper:
      'bg-[linear-gradient(155deg,rgba(110,231,183,0.4),rgba(255,255,255,0.08)_40%,rgba(255,255,255,0))] dark:bg-[linear-gradient(160deg,rgba(110,231,183,0.28),rgba(16,185,129,0.16)_45%,rgba(15,23,42,0.5))]',
    inner: 'border-[#10B981]/70 hover:border-[#10B981]',
    badge: 'metal-badge metal-badge--starter',
    accent: '#10B981',
    ring: 'hover:shadow-[0_18px_50px_-12px_rgba(16,185,129,0.55)]',
  },
  advanced: {
    wrapper:
      'bg-[linear-gradient(155deg,rgba(125,211,252,0.42),rgba(255,255,255,0.08)_40%,rgba(255,255,255,0))] dark:bg-[linear-gradient(160deg,rgba(125,211,252,0.3),rgba(14,165,233,0.16)_45%,rgba(15,23,42,0.5))]',
    inner: 'border-[#0EA5E9]/70 hover:border-[#0EA5E9]',
    badge: 'metal-badge metal-badge--advanced',
    accent: '#0EA5E9',
    ring: 'hover:shadow-[0_18px_50px_-12px_rgba(14,165,233,0.55)]',
  },
  pro: {
    wrapper:
      'bg-[linear-gradient(155deg,rgba(253,224,138,0.7),rgba(255,255,255,0.08)_40%,rgba(255,255,255,0))] dark:bg-[linear-gradient(160deg,rgba(253,224,138,0.3),rgba(245,158,11,0.16)_45%,rgba(30,27,10,0.5))]',
    inner: 'border-[#F59E0B]/75 hover:border-[#F59E0B]',
    badge: 'metal-badge metal-badge--pro',
    accent: '#F59E0B',
    ring: 'hover:shadow-[0_18px_50px_-12px_rgba(245,158,11,0.5)]',
  },
  enterprise: {
    wrapper:
      'bg-[linear-gradient(155deg,rgba(51,65,85,0.5),rgba(241,245,249,0.12)_42%,rgba(255,255,255,0))] dark:bg-[linear-gradient(160deg,rgba(96,165,250,0.26),rgba(30,41,59,0.6)_45%,rgba(15,23,42,0.7))]',
    inner: 'border-[#334155]/80 hover:border-[#475569]',
    badge: 'metal-badge metal-badge--enterprise',
    accent: '#334155',
    ring: 'hover:shadow-[0_18px_50px_-12px_rgba(15,23,42,0.7)]',
  },
}
