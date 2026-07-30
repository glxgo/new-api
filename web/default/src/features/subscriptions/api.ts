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
import type {
  ApiResponse,
  PlanRecord,
  PlanPayload,
  UserSubscriptionRecord,
  CreateUserSubscriptionRequest,
  SubscriptionPayResponse,
  SubscriptionPayRequest,
  SelfSubscriptionData,
  SubscriberSummary,
  SubscriptionRenewalPreview,
  SubscriptionTokenBindingItem,
  BatchSubscriptionBindingPayload,
  UserSubscription,
  SubscriptionConsumptionOrder,
} from './types'

export async function getSubscriptionConsumptionOrder(
  group: string
): Promise<ApiResponse<SubscriptionConsumptionOrder>> {
  const res = await api.get('/api/subscription/self/consumption-order', {
    params: { group },
    disableDuplicate: true,
  })
  return res.data
}

export async function updateSubscriptionConsumptionOrder(data: {
  group: string
  revision: number
  subscription_ids: number[]
}): Promise<ApiResponse<{ revision: number }>> {
  const res = await api.put('/api/subscription/self/consumption-order', data, {
    skipErrorHandler: true,
  })
  return res.data
}

// ============================================================================
// Admin Plan Management
// ============================================================================

export async function getAdminPlans(): Promise<ApiResponse<PlanRecord[]>> {
  const res = await api.get('/api/subscription/admin/plans')
  return res.data
}

export async function createPlan(
  data: PlanPayload
): Promise<ApiResponse<PlanRecord>> {
  const res = await api.post('/api/subscription/admin/plans', data)
  return res.data
}

export async function updatePlan(
  id: number,
  data: PlanPayload
): Promise<ApiResponse<PlanRecord>> {
  const res = await api.put(`/api/subscription/admin/plans/${id}`, data)
  return res.data
}

export async function patchPlanStatus(
  id: number,
  enabled: boolean
): Promise<ApiResponse> {
  const res = await api.patch(`/api/subscription/admin/plans/${id}`, {
    enabled,
  })
  return res.data
}

// ============================================================================
// Admin User Subscription Management
// ============================================================================

export async function getUserSubscriptions(
  userId: number
): Promise<ApiResponse<UserSubscriptionRecord[]>> {
  const res = await api.get(
    `/api/subscription/admin/users/${userId}/subscriptions`
  )
  return res.data
}

export async function getSubscriptionSubscribers(): Promise<
  ApiResponse<SubscriberSummary[]>
> {
  const res = await api.get('/api/subscription/admin/subscribers')
  return res.data
}

export async function createUserSubscription(
  userId: number,
  data: CreateUserSubscriptionRequest
): Promise<ApiResponse<{ message?: string }>> {
  const res = await api.post(
    `/api/subscription/admin/users/${userId}/subscriptions`,
    data
  )
  return res.data
}

export async function invalidateUserSubscription(
  subId: number
): Promise<ApiResponse<{ message?: string }>> {
  const res = await api.post(
    `/api/subscription/admin/user_subscriptions/${subId}/invalidate`
  )
  return res.data
}

export async function deleteUserSubscription(
  subId: number
): Promise<ApiResponse> {
  const res = await api.delete(
    `/api/subscription/admin/user_subscriptions/${subId}`
  )
  return res.data
}

// ============================================================================
// User-facing Subscription Payment
// ============================================================================

export async function paySubscriptionStripe(
  data: SubscriptionPayRequest
): Promise<SubscriptionPayResponse> {
  const res = await api.post('/api/subscription/stripe/pay', data)
  return res.data
}

export async function paySubscriptionCreem(
  data: SubscriptionPayRequest
): Promise<SubscriptionPayResponse> {
  const res = await api.post('/api/subscription/creem/pay', data)
  return res.data
}

export async function paySubscriptionWaffoPancake(
  data: SubscriptionPayRequest
): Promise<SubscriptionPayResponse> {
  const res = await api.post('/api/subscription/waffo-pancake/pay', data)
  return res.data
}

export async function paySubscriptionBalance(
  data: SubscriptionPayRequest
): Promise<SubscriptionPayResponse> {
  const res = await api.post('/api/subscription/balance/pay', data)
  return res.data
}

// Mints a Pancake OnetimeProduct (see controller for the OnetimeProduct vs
// SubscriptionProduct rationale) using persisted creds + StoreID.
export async function createWaffoPancakeSubscriptionProduct(data: {
  name: string
  amount: string
}): Promise<
  ApiResponse<{ product_id: string; product_name: string; store_id: string }>
> {
  const res = await api.post(
    '/api/option/waffo-pancake/subscription-product',
    data
  )
  return res.data
}

// Returns the OnetimeProducts in the saved Pancake store; empty when the
// gateway isn't fully configured.
export async function listWaffoPancakeSubscriptionProductOptions(): Promise<
  ApiResponse<{
    store_id: string
    products: { id: string; name: string; status: string }[]
  }>
> {
  const res = await api.post(
    '/api/option/waffo-pancake/subscription-product-options'
  )
  return res.data
}

export async function paySubscriptionEpay(
  data: SubscriptionPayRequest & { payment_method: string }
): Promise<SubscriptionPayResponse & { url?: string }> {
  const res = await api.post('/api/subscription/epay/pay', data)
  return {
    ...res.data,
    url: res.data.url || (res as unknown as { url?: string }).url,
  }
}

// ============================================================================
// User Self Subscriptions
// ============================================================================

export async function getSelfSubscriptions(): Promise<
  ApiResponse<UserSubscriptionRecord[]>
> {
  const res = await api.get('/api/subscription/self')
  return res.data
}

export async function getSelfSubscriptionFull(): Promise<
  ApiResponse<SelfSubscriptionData>
> {
  const res = await api.get('/api/subscription/self')
  return res.data
}

export async function updateSubscriptionRemark(
  subscriptionId: number,
  remark: string
): Promise<ApiResponse<UserSubscription>> {
  const res = await api.patch(
    `/api/subscription/self/instances/${subscriptionId}/remark`,
    { remark }
  )
  return res.data
}

export async function getSubscriptionRenewalPreview(
  subscriptionId: number
): Promise<ApiResponse<SubscriptionRenewalPreview>> {
  const res = await api.get(
    `/api/subscription/self/instances/${subscriptionId}/renewal-preview`
  )
  return res.data
}

export async function getSubscriptionTokenBindings(
  subscriptionId: number
): Promise<ApiResponse<SubscriptionTokenBindingItem[]>> {
  const res = await api.get(
    `/api/subscription/self/instances/${subscriptionId}/keys`
  )
  return res.data
}

export async function replaceSubscriptionTokenBindings(
  subscriptionId: number,
  payload: BatchSubscriptionBindingPayload
): Promise<ApiResponse> {
  const res = await api.put(
    `/api/subscription/self/instances/${subscriptionId}/keys`,
    payload
  )
  return res.data
}

export async function getPublicPlans(): Promise<ApiResponse<PlanRecord[]>> {
  const res = await api.get('/api/subscription/plans')
  return res.data
}

// 套餐订阅页顶部全局介绍(富文本 markdown), 后台套餐管理页编辑
// 兼容旧后端: 线上未部署 /intro 时静默降级为空, 避免本地预览弹 404 toast.
export async function getSubscriptionIntro(): Promise<
  ApiResponse<{ intro: string }>
> {
  try {
    const res = await api.get('/api/subscription/intro', {
      skipErrorHandler: true,
    })
    return res.data
  } catch (error: unknown) {
    const status = (error as { response?: { status?: number } })?.response
      ?.status
    if (status === 404) {
      return { success: true, data: { intro: '' } }
    }
    throw error
  }
}

export async function updateBillingPreference(
  preference: string
): Promise<ApiResponse<{ billing_preference?: string }>> {
  const res = await api.put('/api/subscription/self/preference', {
    billing_preference: preference,
  })
  return res.data
}

export async function getGroups(): Promise<ApiResponse<string[]>> {
  const res = await api.get('/api/group')
  return res.data
}
