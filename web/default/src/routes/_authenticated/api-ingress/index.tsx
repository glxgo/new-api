import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { APIIngress } from '@/features/api-ingress'

export const Route = createFileRoute('/_authenticated/api-ingress/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN)
      throw redirect({ to: '/403' })
  },
  component: APIIngress,
})
