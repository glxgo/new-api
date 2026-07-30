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
// Subscription Plan Schema & Types
// ============================================================================

export const subscriptionPlanSchema = z.object({
  id: z.number(),
  title: z.string(),
  subtitle: z.string().optional(),
  suitable_for: z.string().optional(),
  price_amount: z.number(),
  currency: z.string().default('USD'),
  duration_unit: z.enum(['year', 'month', 'week', 'day', 'hour', 'custom']),
  duration_value: z.number(),
  custom_seconds: z.number().optional(),
  quota_reset_period: z.enum(['never', 'daily', 'weekly', 'monthly', 'custom']),
  quota_reset_custom_seconds: z.number().optional(),
  enabled: z.boolean(),
  sort_order: z.number(),
  allow_balance_pay: z.boolean().optional().default(true),
  lucky_card_grant_count: z.number().optional().default(0),
  lucky_card_on_reset: z.boolean().optional().default(false),
  max_purchase_per_user: z.number(),
  total_amount: z.number(),
  upgrade_group: z.string().optional(),
  allowed_group: z.string().optional(),
  renewal_plan_id: z.number().nullable().optional(),
  number_pool: z.string().optional(),
  model_limit: z.string().optional(),
  plan_version: z.enum(['starter', 'advanced', 'pro', 'enterprise']).optional(),
  recommended: z.boolean().optional(),
  min_ratio: z.number().optional(),
  amount_cap: z.number().optional(),
  description: z.string().optional(),
  stripe_price_id: z.string().optional(),
  creem_product_id: z.string().optional(),
  waffo_pancake_product_id: z.string().optional(),
})

export type SubscriptionPlan = z.infer<typeof subscriptionPlanSchema>

export interface PlanRecord {
  plan: SubscriptionPlan
  subscriber_count?: number
  active_count?: number
}

export type SubscriberSummary = {
  user_id: number
  username: string
  total_count: number
  active_count: number
}

// ============================================================================
// User Subscription Schema & Types
// ============================================================================

export const userSubscriptionSchema = z.object({
  id: z.number(),
  user_id: z.number(),
  plan_id: z.number(),
  plan_title: z.string().optional(),
  remark: z.string().optional(),
  renewed_from_id: z.number().nullable().optional(),
  status: z.string(),
  source: z.string().optional(),
  start_time: z.number(),
  end_time: z.number(),
  amount_total: z.number(),
  amount_used: z.number(),
  amount_cap: z.number().optional(),
  amount_cap_used: z.number().optional(),
  allowed_group: z.string().optional(),
  next_reset_time: z.number().optional(),
  lucky_card_disabled: z.boolean().optional(),
})

export type UserSubscription = z.infer<typeof userSubscriptionSchema>

export interface UserSubscriptionRecord {
  subscription: UserSubscription
}

export interface SubscriptionConsumptionPriority {
  id: number
  subscription_id: number
  priority: number
  revision: number
}

export interface SubscriptionConsumptionOrder {
  group: string
  revision: number
  subscriptions: UserSubscription[]
  order: SubscriptionConsumptionPriority[]
}

// ============================================================================
// API Request/Response Types
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface PlanPayload {
  plan: Partial<SubscriptionPlan>
}

export interface SubscriptionPayRequest {
  plan_id: number
  payment_method?: string
  renew_from_subscription_id?: number
}

export interface RenewalBindingChange {
  token_id: number
  token_name: string
  from_group: string
  to_group: string
  effective_at: number
  applied_immediately?: boolean
}

export interface SubscriptionRenewalPreview {
  from_subscription: UserSubscription
  plan: SubscriptionPlan
  is_replacement: boolean
  binding_changes: RenewalBindingChange[]
  start_time: number
  end_time: number
}

export interface SubscriptionTokenBindingItem {
  id: number
  name: string
  group: string
  status: number
  subscription_mode: 'auto' | 'instance'
  subscription_id: number
  subscription_allow_renewal: boolean
  subscription_allow_same_group: boolean
  subscription_allow_wallet: boolean
  subscription_wallet_limit: number
  subscription_wallet_used: number
  planned_subscription_id: number
  planned_subscription_effective: number
  compatible: boolean
  incompatibility_reason: string
}

export interface BatchSubscriptionBindingPayload {
  token_ids: number[]
  subscription_allow_renewal: boolean
  subscription_allow_same_group: boolean
  subscription_allow_wallet: boolean
  subscription_wallet_limit: number
  keep_planned_token_ids?: number[]
}

export interface SubscriptionPayResponse {
  success: boolean
  message?: string
  data?: {
    // Stripe-style hosted checkout link.
    pay_link?: string
    // Waffo Pancake / Creem hosted checkout URL.
    checkout_url?: string
    // Pancake-only: order metadata + self-service buyer session token,
    // surfaced for future flows (refund / cancel from new-api's own UI).
    session_id?: string
    expires_at?: number | string
    order_id?: string
    token?: string
    token_expires_at?: number | string
  }
  url?: string
}

export interface CreateUserSubscriptionRequest {
  plan_id: number
}

// ============================================================================
// Self Subscription Data (user-facing)
// ============================================================================

export interface SelfSubscriptionData {
  billing_preference: string
  subscriptions: UserSubscriptionRecord[]
  all_subscriptions: UserSubscriptionRecord[]
}

// ============================================================================
// Dialog Types
// ============================================================================

export type SubscriptionsDialogType = 'create' | 'update' | 'toggle-status'
