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

// ============================================================================
// API Key Schema & Types
// ============================================================================

export const apiKeyRouteStepSchema = z.object({
  id: z.number().optional().default(0),
  position: z.number(),
  group: z.string(),
  funding_source: z
    .enum(['wallet', 'subscription', 'virtual_membership'])
    .optional(),
  selection_mode: z.enum(['auto', 'instance']),
  source_id: z.number(),
})

export type ApiKeyRouteStep = Omit<
  z.infer<typeof apiKeyRouteStepSchema>,
  'id'
> & { id?: number }

export const apiKeySchema = z.object({
  id: z.number(),
  name: z.string(),
  key: z.string(),
  status: z.number(), // 1: enabled, 2: disabled, 3: expired, 4: exhausted
  remain_quota: z.number(),
  used_quota: z.number(),
  unlimited_quota: z.boolean(),
  expired_time: z.number(), // -1 for never expires
  created_time: z.number(),
  accessed_time: z.number(),
  group: z.string().nullish().default(''),
  cross_group_retry: z
    .preprocess((v) => {
      if (v === 1) return true
      if (v === 0) return false
      return v
    }, z.boolean())
    .optional()
    .default(false),
  routing_mode: z.enum(['single', 'custom']).optional().default('single'),
  routing_revision: z.number().optional().default(0),
  route_steps: z.array(apiKeyRouteStepSchema).optional().default([]),
  subscription_mode: z.enum(['auto', 'instance']).optional().default('auto'),
  subscription_id: z.number().optional().default(0),
  subscription_allow_renewal: z.boolean().optional().default(false),
  subscription_allow_same_group: z.boolean().optional().default(false),
  subscription_allow_wallet: z.boolean().optional().default(false),
  subscription_wallet_limit: z.number().optional().default(0),
  subscription_wallet_used: z.number().optional().default(0),
  subscription_wallet_cycle_id: z.number().optional().default(0),
  planned_subscription_id: z.number().optional().default(0),
  planned_subscription_group: z.string().optional().default(''),
  planned_subscription_effective: z.number().optional().default(0),
  virtual_membership_id: z.number().optional().default(0),
  virtual_membership_mode: z
    .enum(['auto', 'instance'])
    .optional()
    .default('instance'),
  model_limits_enabled: z.boolean(),
  model_limits: z.string().nullish().default(''),
  allow_ips: z.string().nullish().default(''),
})

export type ApiKey = z.infer<typeof apiKeySchema>

// ============================================================================
// API Request/Response Types
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface GetApiKeysParams {
  p?: number
  size?: number
}

export interface GetApiKeysResponse {
  success: boolean
  message?: string
  data?: {
    items: ApiKey[]
    total: number
    page: number
    page_size: number
  }
}

export interface SearchApiKeysParams {
  keyword?: string
  token?: string
  p?: number
  size?: number
}

export interface ApiKeyFormData {
  name: string
  remain_quota: number
  expired_time: number
  unlimited_quota: boolean
  model_limits_enabled: boolean
  model_limits: string
  allow_ips: string
  group: string
  cross_group_retry: boolean
  routing_mode: 'single' | 'custom'
  routing_revision: number
  route_steps: ApiKeyRouteStep[]
  subscription_mode: 'auto' | 'instance'
  subscription_id: number
  subscription_allow_renewal: boolean
  subscription_allow_same_group: boolean
  subscription_allow_wallet: boolean
  subscription_wallet_limit: number
  cancel_planned_subscription: boolean
  virtual_membership_id: number
  virtual_membership_mode: 'auto' | 'instance'
}

export interface ApiKeySubscriptionHistory {
  id: number
  token_id: number
  actor_type: string
  action: string
  from_subscription_id: number
  to_subscription_id: number
  from_group: string
  to_group: string
  continuation_summary: string
  reason: string
  created_at: number
}

// ============================================================================
// Dialog Types
// ============================================================================

export type ApiKeysDialogType =
  | 'create'
  | 'update'
  | 'delete'
  | 'batch-delete'
  | 'cc-switch'
