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
import { api } from '@/lib/api'
import type { ApiResponse } from '@/features/subscriptions/types'
import type {
  AdminVirtualMembership,
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
  renew_from_membership_id?: number
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
  renew_from_membership_id?: number
}): Promise<ApiResponse<Record<string, string>> & { url?: string }> {
  const res = await api.post('/api/virtual-membership/epay/pay', data)
  return {
    ...res.data,
    // Keep the virtual-membership payment flow compatible with the
    // subscription payment API when an adapter exposes the URL on the
    // Axios response instead of inside response.data.
    url: res.data.url || (res as unknown as { url?: string }).url,
  }
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

export async function resetAdminVirtualMemberships(
  data: {
    membership_id?: number
    user_id?: number
    plan_code?: string
  } = {}
): Promise<ApiResponse<{ affected: number; next_reset_at: number }>> {
  const res = await api.post('/api/virtual-membership/admin/reset', data)
  return res.data
}

export async function getAdminVirtualMemberships(): Promise<
  ApiResponse<AdminVirtualMembership[]>
> {
  const res = await api.get('/api/virtual-membership/admin/memberships')
  return res.data
}

export async function grantAdminVirtualMembership(data: {
  user_id: number
  plan_id: number
  group_size: number
}): Promise<
  ApiResponse<{ order_id: number; membership: UserVirtualMembership }>
> {
  const res = await api.post('/api/virtual-membership/admin/memberships', data)
  return res.data
}

export async function deleteAdminVirtualMembership(
  membershipId: number
): Promise<ApiResponse<{ unbound_tokens: number }>> {
  const res = await api.delete(
    `/api/virtual-membership/admin/memberships/${membershipId}`
  )
  return res.data
}

export async function renewAdminVirtualMembership(
  membershipId: number
): Promise<
  ApiResponse<{ order_id: number; membership: UserVirtualMembership }>
> {
  const res = await api.post(
    `/api/virtual-membership/admin/memberships/${membershipId}/renew`
  )
  return res.data
}

export async function setAdminVirtualMembershipHidden(
  membershipId: number,
  hidden: boolean
): Promise<ApiResponse<{ hidden: boolean }>> {
  const res = await api.patch(
    `/api/virtual-membership/admin/memberships/${membershipId}/visibility`,
    { hidden }
  )
  return res.data
}
