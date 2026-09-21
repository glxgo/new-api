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
  InvoiceApplication,
  InvoiceEligibleOrder,
  InvoiceListResponse,
} from './types'

interface ApiResponse<T = unknown> {
  success?: boolean
  message?: string
  data?: T
}

export async function getEligibleInvoiceOrders(): Promise<
  ApiResponse<InvoiceEligibleOrder[]>
> {
  const res = await api.get('/api/invoices/eligible-orders')
  return res.data
}

export async function getUserInvoiceApplications(): Promise<
  ApiResponse<InvoiceApplication[]>
> {
  const res = await api.get('/api/invoices/self')
  return res.data
}

export async function createInvoiceApplication(payload: {
  top_up_ids: number[]
  subject_type: string
  title: string
  taxpayer_id: string
  email: string
}): Promise<ApiResponse<InvoiceApplication>> {
  const res = await api.post('/api/invoices', payload)
  return res.data
}

export async function getAdminInvoiceApplications(
  params: {
    page?: number
    page_size?: number
    status?: string
  } = {}
): Promise<ApiResponse<InvoiceListResponse>> {
  const page = params.page ?? 1
  const pageSize = params.page_size ?? 20
  const status = params.status ?? 'all'
  const res = await api.get(
    `/api/invoices/admin?page=${page}&page_size=${pageSize}&status=${encodeURIComponent(status)}`
  )
  return res.data
}

export async function approveInvoiceApplication(
  id: number,
  remark = ''
): Promise<ApiResponse<InvoiceApplication>> {
  const res = await api.post(`/api/invoices/admin/${id}/approve`, { remark })
  return res.data
}

export async function rejectInvoiceApplication(
  id: number,
  remark = ''
): Promise<ApiResponse<InvoiceApplication>> {
  const res = await api.post(`/api/invoices/admin/${id}/reject`, { remark })
  return res.data
}
