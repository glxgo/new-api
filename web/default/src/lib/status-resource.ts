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
