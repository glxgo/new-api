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
import type { LogOtherData } from '../types'

export interface UserVisibleRequestRouting {
  requestPath?: string
  conversionChain: string[]
}

export interface UserVisibleModelMapping {
  requestModel: string
  actualModel: string
}

export function getUserVisibleModelMapping(
  requestModel: string,
  other: LogOtherData | null | undefined
): UserVisibleModelMapping | null {
  const actualModel = other?.upstream_model_name?.trim()
  if (!other?.is_model_mapped || !actualModel) return null
  return { requestModel, actualModel }
}

// Request path and format-conversion names describe the caller's own request
// and are safe to show to its owner. Channel IDs/names, retry chains, affinity
// keys and all other administrator diagnostics deliberately remain excluded.
export function getUserVisibleRequestRouting(
  logType: number,
  other: LogOtherData | null | undefined
): UserVisibleRequestRouting | null {
  if (!other || logType === 1 || logType === 6) return null

  const requestPath = other.request_path?.trim() || undefined
  const conversionChain = Array.isArray(other.request_conversion)
    ? other.request_conversion
        .filter((item): item is string => typeof item === 'string')
        .map((item) => item.trim())
        .filter(Boolean)
    : []

  if (!requestPath && conversionChain.length === 0) return null
  return { requestPath, conversionChain }
}
