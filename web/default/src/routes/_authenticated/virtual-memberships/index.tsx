import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { VirtualMemberships } from '@/features/virtual-memberships'

export const Route = createFileRoute('/_authenticated/virtual-memberships/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN)
      throw redirect({ to: '/403' })
  },
  component: VirtualMemberships,
})
