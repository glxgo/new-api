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
import { z } from 'zod'
import type { TFunction } from 'i18next'
import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'
import { type ApiKeyFormData, type ApiKey } from '../types'

// ============================================================================
// Form Schema
// ============================================================================

export function getApiKeyFormSchema(t: TFunction) {
  return z
    .object({
      name: z.string().min(1, t('Please enter a name')),
      remain_quota_dollars: z.number().optional(),
      expired_time: z.date().optional(),
      unlimited_quota: z.boolean(),
      model_limits: z.array(z.string()),
      allow_ips: z.string().optional(),
      group: z.string().trim().min(1, t('Please select a group')),
      cross_group_retry: z.boolean().optional(),
      subscription_mode: z.enum(['auto', 'instance']),
      subscription_id: z.number(),
      subscription_allow_renewal: z.boolean(),
      subscription_allow_same_group: z.boolean(),
      subscription_allow_wallet: z.boolean(),
      subscription_wallet_limit_dollars: z.number(),
      keep_planned_subscription: z.boolean(),
      tokenCount: z.number().min(1).optional(),
    })
    .superRefine((data, ctx) => {
      if (
        !data.unlimited_quota &&
        (data.remain_quota_dollars === undefined ||
          data.remain_quota_dollars < 0)
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['remain_quota_dollars'],
          message: t('Quota must be zero or greater'),
        })
      }
      if (data.subscription_mode === 'instance' && data.subscription_id <= 0) {
        ctx.addIssue({
          code: 'custom',
          path: ['subscription_id'],
          message: t('Please select a subscription instance'),
        })
      }
      if (
        data.subscription_mode === 'instance' &&
        data.subscription_allow_wallet &&
        data.subscription_wallet_limit_dollars <= 0
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['subscription_wallet_limit_dollars'],
          message: t('Wallet fallback limit must be greater than zero'),
        })
      }
    })
}

export type ApiKeyFormValues = z.infer<ReturnType<typeof getApiKeyFormSchema>>

// ============================================================================
// Form Defaults
// ============================================================================

export const API_KEY_FORM_DEFAULT_VALUES: ApiKeyFormValues = {
  name: '',
  remain_quota_dollars: 10,
  expired_time: undefined,
  unlimited_quota: true,
  model_limits: [],
  allow_ips: '',
  group: '',
  cross_group_retry: false,
  subscription_mode: 'auto',
  subscription_id: 0,
  subscription_allow_renewal: true,
  subscription_allow_same_group: false,
  subscription_allow_wallet: false,
  subscription_wallet_limit_dollars: 0,
  keep_planned_subscription: false,
  tokenCount: 1,
}

export function getApiKeyFormDefaultValues(
  defaultUseAutoGroup: boolean
): ApiKeyFormValues {
  void defaultUseAutoGroup
  return {
    ...API_KEY_FORM_DEFAULT_VALUES,
  }
}

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform form data to API payload
 */
export function transformFormDataToPayload(
  data: ApiKeyFormValues
): ApiKeyFormData {
  return {
    name: data.name,
    remain_quota: data.unlimited_quota
      ? 0
      : parseQuotaFromDollars(data.remain_quota_dollars || 0),
    expired_time: data.expired_time
      ? Math.floor(data.expired_time.getTime() / 1000)
      : -1,
    unlimited_quota: data.unlimited_quota,
    model_limits_enabled: data.model_limits.length > 0,
    model_limits: data.model_limits.join(','),
    allow_ips: data.allow_ips || '',
    group: data.group.trim(),
    cross_group_retry: data.group === 'auto' ? !!data.cross_group_retry : false,
    subscription_mode: data.subscription_mode,
    subscription_id:
      data.subscription_mode === 'instance' ? data.subscription_id : 0,
    subscription_allow_renewal:
      data.subscription_mode === 'instance' && data.subscription_allow_renewal,
    subscription_allow_same_group:
      data.subscription_mode === 'instance' &&
      data.subscription_allow_same_group,
    subscription_allow_wallet:
      data.subscription_mode === 'instance' && data.subscription_allow_wallet,
    subscription_wallet_limit:
      data.subscription_mode === 'instance' && data.subscription_allow_wallet
        ? parseQuotaFromDollars(data.subscription_wallet_limit_dollars)
        : 0,
    cancel_planned_subscription:
      data.subscription_mode === 'auto' && !data.keep_planned_subscription,
  }
}

/**
 * Transform API key data to form defaults
 */
export function transformApiKeyToFormDefaults(
  apiKey: ApiKey
): ApiKeyFormValues {
  return {
    name: apiKey.name,
    remain_quota_dollars: apiKey.unlimited_quota
      ? 0
      : quotaUnitsToDollars(apiKey.remain_quota),
    expired_time:
      apiKey.expired_time > 0
        ? new Date(apiKey.expired_time * 1000)
        : undefined,
    unlimited_quota: apiKey.unlimited_quota,
    model_limits: apiKey.model_limits
      ? apiKey.model_limits.split(',').filter(Boolean)
      : [],
    allow_ips: apiKey.allow_ips || '',
    group: apiKey.group || '',
    cross_group_retry: !!apiKey.cross_group_retry,
    subscription_mode: apiKey.subscription_mode || 'auto',
    subscription_id: apiKey.subscription_id || 0,
    subscription_allow_renewal: !!apiKey.subscription_allow_renewal,
    subscription_allow_same_group: !!apiKey.subscription_allow_same_group,
    subscription_allow_wallet: !!apiKey.subscription_allow_wallet,
    subscription_wallet_limit_dollars: quotaUnitsToDollars(
      apiKey.subscription_wallet_limit || 0
    ),
    keep_planned_subscription:
      apiKey.subscription_mode === 'auto' && apiKey.planned_subscription_id > 0,
    tokenCount: 1,
  }
}
