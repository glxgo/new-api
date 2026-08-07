import type { ApiResponse } from '@/features/subscriptions/types'

export interface VirtualMembershipVariant {
  group_size: number
  label: string
  original_price_amount: number
  price_amount: number
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
