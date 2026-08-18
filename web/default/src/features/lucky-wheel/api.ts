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
  LuckyAdminDraw,
  LuckyCard,
  LuckyDraw,
  LuckyRuleSet,
  LuckyWheelStatus,
  PageResult,
} from './types'

interface ApiResponse<T> {
  success: boolean
  message?: string
  data: T
}

export async function getLuckyWheelStatus() {
  return (
    await api.get<ApiResponse<LuckyWheelStatus>>('/api/lucky-wheel/status', {
      skipErrorHandler: true,
    })
  ).data
}

export async function getLuckyCards() {
  return (
    await api.get<ApiResponse<PageResult<LuckyCard>>>(
      '/api/lucky-wheel/cards',
      {
        params: { page: 1, page_size: 100 },
        disableDuplicate: true,
        skipErrorHandler: true,
      }
    )
  ).data
}

export async function getLuckyDraws(page = 1, pageSize = 10) {
  return (
    await api.get<ApiResponse<PageResult<LuckyDraw>>>(
      '/api/lucky-wheel/draws',
      {
        params: { page, page_size: pageSize },
        disableDuplicate: true,
        skipErrorHandler: true,
      }
    )
  ).data
}

export async function getLuckyRules() {
  return (
    await api.get<ApiResponse<LuckyRuleSet[]>>('/api/lucky-wheel/rules', {
      skipErrorHandler: true,
    })
  ).data
}

export async function createLuckyDraw(cardId: number, idempotencyKey: string) {
  return (
    await api.post<ApiResponse<LuckyDraw>>('/api/lucky-wheel/draws', {
      card_id: cardId,
      idempotency_key: idempotencyKey,
    })
  ).data
}

export async function getLuckyAdminOverview() {
  return (
    await api.get<
      ApiResponse<{
        campaign: LuckyWheelStatus['campaign']
        active_rule: LuckyRuleSet
        cards: Array<{ status: string; count: number }>
        draws: number
      }>
    >('/api/lucky-wheel/admin/overview')
  ).data
}

export async function getLuckyAdminRuleSets() {
  return (
    await api.get<ApiResponse<LuckyRuleSet[]>>(
      '/api/lucky-wheel/admin/rule-sets',
      { disableDuplicate: true }
    )
  ).data
}

export async function createLuckyRuleSet(data: {
  base_rule_set_id: number
  subscription_pool: string
  recharge_pool: string
}) {
  return (
    await api.post<ApiResponse<LuckyRuleSet>>(
      '/api/lucky-wheel/admin/rule-sets',
      data
    )
  ).data
}

export async function activateLuckyRuleSet(id: number) {
  return (
    await api.post<ApiResponse<LuckyRuleSet>>(
      `/api/lucky-wheel/admin/rule-sets/${id}/activate`
    )
  ).data
}

export interface LuckyAdminDrawFilters {
  keyword?: string
  prize_type?: string
  status?: string
  start_time?: number
  end_time?: number
  page?: number
  page_size?: number
}

export async function getLuckyAdminDraws(filters: LuckyAdminDrawFilters = {}) {
  return (
    await api.get<ApiResponse<PageResult<LuckyAdminDraw>>>(
      '/api/lucky-wheel/admin/draws',
      {
        params: filters,
        disableDuplicate: true,
      }
    )
  ).data
}

export async function setLuckyIssuancePaused(paused: boolean, reason: string) {
  return (
    await api.post<ApiResponse<null>>(
      `/api/lucky-wheel/admin/${paused ? 'pause' : 'resume'}-issuance`,
      { reason }
    )
  ).data
}

export async function setLuckyDrawPaused(paused: boolean, reason: string) {
  return (
    await api.post<ApiResponse<null>>(
      `/api/lucky-wheel/admin/${paused ? 'pause' : 'resume'}-draw`,
      { reason }
    )
  ).data
}

export async function compensateLuckyCards(data: {
  user_id: number
  count: number
  pool_type: 'recharge' | 'subscription'
  source_subscription_id?: number
  ticket: string
}) {
  return (
    await api.post<ApiResponse<LuckyCard[]>>(
      '/api/lucky-wheel/admin/cards/compensate',
      data
    )
  ).data
}

export interface LuckyAdminCardsResult extends PageResult<LuckyCard> {
  user: {
    id: number
    username: string
    display_name: string
  } | null
  status_counts: Array<{ status: string; count: number }>
}

export async function getLuckyAdminCards(
  userId: number,
  page = 1,
  pageSize = 10
) {
  return (
    await api.get<ApiResponse<LuckyAdminCardsResult>>(
      '/api/lucky-wheel/admin/cards',
      {
        params: { user_id: userId, page, page_size: pageSize },
        disableDuplicate: true,
      }
    )
  ).data
}

export async function revokeLuckyUserCards(data: {
  user_id: number
  reason: string
}) {
  return (
    await api.post<
      ApiResponse<{
        revoked_cards: number
        preserved_draw_history: boolean
      }>
    >('/api/lucky-wheel/admin/cards/revoke-user', data)
  ).data
}

export async function reverseLuckySource(data: {
  source_type: 'wallet_topup' | 'subscription_order'
  trade_no: string
  reason: string
}) {
  return (
    await api.post<
      ApiResponse<{
        event_created: boolean
        revoked_cards: number
        review_cards: number
        review_draws: number
      }>
    >('/api/lucky-wheel/admin/source-reversals', data)
  ).data
}
