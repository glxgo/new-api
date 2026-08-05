import { api } from '@/lib/api'
import type { ApiResponse } from '@/features/subscriptions/types'
import type {
  UserVirtualMembership,
  VirtualMembershipPageData,
  VirtualMembershipPlan,
} from './types'

export async function getVirtualMembershipPage(): Promise<
  ApiResponse<VirtualMembershipPageData>
> {
  const res = await api.get('/api/virtual-membership/page')
  return res.data
}

export async function purchaseVirtualMembership(data: {
  plan_id: number
  group_size: number
}): Promise<
  ApiResponse<{ order: unknown; membership: UserVirtualMembership }>
> {
  const res = await api.post('/api/virtual-membership/balance/pay', data)
  return res.data
}

export async function payVirtualMembershipEpay(data: {
  plan_id: number
  group_size: number
  payment_method: string
}): Promise<ApiResponse<Record<string, string>> & { url?: string }> {
  const res = await api.post('/api/virtual-membership/epay/pay', data)
  return res.data
}

export async function getAdminVirtualMembershipPlans(): Promise<
  ApiResponse<VirtualMembershipPlan[]>
> {
  const res = await api.get('/api/virtual-membership/admin/plans')
  return res.data
}

export async function saveAdminVirtualMembershipPlan(
  data: Partial<VirtualMembershipPlan> & { id?: number }
): Promise<ApiResponse<VirtualMembershipPlan>> {
  const { id, ...payload } = data
  const res = id
    ? await api.put(`/api/virtual-membership/admin/plans/${id}`, payload)
    : await api.post('/api/virtual-membership/admin/plans', payload)
  return res.data
}

export async function getAdminVirtualMembershipSetting(): Promise<
  ApiResponse<{ announcement: string; enabled: boolean }>
> {
  const res = await api.get('/api/virtual-membership/admin/setting')
  return res.data
}

export async function saveAdminVirtualMembershipSetting(data: {
  announcement: string
  enabled: boolean
}): Promise<ApiResponse> {
  const res = await api.put('/api/virtual-membership/admin/setting', data)
  return res.data
}

export async function resetAdminVirtualMemberships(): Promise<
  ApiResponse<{ affected: number; next_reset_at: number }>
> {
  const res = await api.post('/api/virtual-membership/admin/reset')
  return res.data
}
