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
export interface InvoiceEligibleOrder {
  id: number
  trade_no: string
  amount: number
  currency: string
  payment_method: string
  payment_time: number
}

export interface InvoiceApplicationOrder {
  id: number
  top_up_id: number
  trade_no: string
  amount: number
  currency: string
  payment_method: string
  payment_time: number
}

export type InvoiceStatus = 'pending' | 'approved' | 'rejected'

export interface InvoiceApplication {
  id: number
  user_id: number
  subject_type: 'company' | 'personal' | 'other'
  subject_label: string
  title: string
  taxpayer_id: string
  email: string
  total_amount: number
  currency: string
  status: InvoiceStatus
  remark: string
  handler_id: number
  handler_name: string
  handled_at: number
  created_at: number
  updated_at: number
  orders: InvoiceApplicationOrder[]
}

export interface InvoiceListResponse {
  data: InvoiceApplication[]
  total: number
}
