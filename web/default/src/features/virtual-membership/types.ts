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
import type { ApiResponse } from '@/features/subscriptions/types'

export interface VirtualMembershipVariant {
  group_size: number
  label: string
  original_price_amount: number
  price_amount: number
  fixed_profit_amount: number
  weekly_quota: number
  five_hour_quota: number
  concurrency_limit: number
  rpm_limit: number
}

export interface VirtualMembershipPlan {
  id: number
  code: string
  title: string
  subtitle?: string
  description?: string
  original_price_amount: number
  price_amount: number
  two_group_original_price: number
  two_group_price: number
  three_group_original_price: number
  three_group_price: number
  four_group_original_price: number
  four_group_price: number
  fixed_profit_amount: number
  currency: string
  duration_days: number
  weekly_quota: number
  five_hour_enabled: boolean
  five_hour_quota: number
  concurrency_limit: number
  rpm_limit: number
  recommended: boolean
  enabled: boolean
  sort_order: number
  allowed_models?: string
  allowed_group?: string
  variants: VirtualMembershipVariant[]
}

export interface UserVirtualMembership {
  id: number
  plan_id: number
  order_id: number
  plan_title: string
  plan_code: string
  group_size: number
  weekly_quota: number
  weekly_used: number
  weekly_remaining: number
  weekly_percent: number
  five_hour_enabled: boolean
  five_hour_quota: number
  five_hour_used: number
  five_hour_remaining: number
  five_hour_percent: number
  concurrency_limit: number
  rpm_limit: number
  weekly_reset_at: number
  five_hour_reset_at: number
  start_time: number
  end_time: number
  status: string
  allowed_models?: string
  allowed_group?: string
}

export interface AdminVirtualMembership extends UserVirtualMembership {
  user_id: number
  username: string
  display_name?: string
  email?: string
  user_deleted?: boolean
}

export interface VirtualMembershipPageData {
  announcement: string
  enabled: boolean
  plans: VirtualMembershipPlan[]
  memberships: UserVirtualMembership[]
  epay_enabled?: boolean
  epay_methods?: { type: string; name?: string }[]
}

export type VirtualMembershipPageResponse =
  ApiResponse<VirtualMembershipPageData>
