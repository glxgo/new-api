import { createFileRoute } from '@tanstack/react-router'
import { VirtualMembership } from '@/features/virtual-membership'

export const Route = createFileRoute('/_authenticated/virtual-membership/')({
  component: VirtualMembership,
})
