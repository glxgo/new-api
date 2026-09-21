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

export interface ClientIdentity {
  client_key: string
  family: string
  variant: string
  display_name: string
  user_agent: string
  truncated: boolean
  request_count: number
  last_seen: number
  status: string
  revision: number
}
export interface ClientPolicy {
  group_name: string
  is_coding: boolean
  supported_clients: string[]
  revision: number
}
export interface ClientAudit {
  id: number
  operator_id: number
  status?: string
  reason: string
  created_at: number
  previous_policy?: string
  policy?: string
}
export interface ClientPage<T> {
  items: T[]
  total: number
}

export async function clientRequest<T>(
  path: string,
  body?: unknown,
  method = 'post'
): Promise<T> {
  const response =
    body === undefined
      ? await api.get(`/api/clients${path}`)
      : await api.request({ url: `/api/clients${path}`, method, data: body })
  if (!response.data.success)
    throw new Error(response.data.message || 'Request failed')
  return response.data.data
}
