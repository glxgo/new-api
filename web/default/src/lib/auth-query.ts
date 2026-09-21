import type { QueryClient } from '@tanstack/react-query'
import { useAuthStore } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'

// Memory only: a full reload always verifies the session again. Route entry
// after logout also forces a fetch, even when the same user signs in again.
export function getValidatedSelf(
  queryClient: QueryClient,
  userId: number,
  force = false
) {
  return queryClient.fetchQuery({
    queryKey: ['auth', 'self', userId],
    queryFn: async () => {
      const response = await getSelf()
      if (
        response?.success &&
        response.data &&
        useAuthStore.getState().auth.user?.id === userId
      ) {
        // Update only after a network response: replaying cached profile data
        // could overwrite a more recent wallet/profile update in the store.
        useAuthStore.getState().auth.setUser(response.data)
      }
      return response
    },
    staleTime: force ? 0 : 15_000,
    gcTime: 30_000,
    retry: false,
  })
}
