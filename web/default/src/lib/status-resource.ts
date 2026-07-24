import { getStatus } from '@/lib/api'
import { createTimedRequestCache } from './status-request-cache'

const STATUS_STORAGE_KEY = 'status'
const STATUS_REQUEST_TTL_MS = 30_000

export type SharedStatus = Record<string, unknown>

export function readCachedStatus(): SharedStatus | null {
  try {
    if (typeof window === 'undefined') return null
    const raw = window.localStorage.getItem(STATUS_STORAGE_KEY)
    return raw ? (JSON.parse(raw) as SharedStatus) : null
  } catch {
    return null
  }
}

function persistStatus(status: SharedStatus | null): void {
  try {
    if (typeof window !== 'undefined' && status) {
      window.localStorage.setItem(STATUS_STORAGE_KEY, JSON.stringify(status))
    }
  } catch {
    /* empty */
  }
}

const statusRequest = createTimedRequestCache<SharedStatus | null>({
  ttlMs: STATUS_REQUEST_TTL_MS,
  load: async () => (await getStatus()) as SharedStatus | null,
  onCache: persistStatus,
})

export function getSharedStatus(): Promise<SharedStatus | null> {
  return statusRequest.get()
}

export function invalidateSharedStatus(): void {
  statusRequest.clear()
}
