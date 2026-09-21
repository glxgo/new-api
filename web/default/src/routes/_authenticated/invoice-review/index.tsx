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
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { SectionPageLayout } from '@/components/layout'
import { InvoiceReviewTable } from '@/features/invoices/components/invoice-review-table'

export const Route = createFileRoute('/_authenticated/invoice-review/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.SUPER_ADMIN)
      throw redirect({ to: '/403' })
  },
  component: InvoiceReview,
})

function InvoiceReview() {
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>发票审核</SectionPageLayout.Title>
      <SectionPageLayout.Description>
        审核客户发票申请并记录处理备注。
      </SectionPageLayout.Description>
      <SectionPageLayout.Content>
        <div className='mx-auto w-full max-w-7xl'>
          <InvoiceReviewTable />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
