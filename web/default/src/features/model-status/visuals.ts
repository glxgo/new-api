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
export const HEALTHY_AVAILABILITY_THRESHOLD = 90
export const UNSTABLE_AVAILABILITY_THRESHOLD = 80
export const AVAILABILITY_BAR_HEIGHT_FLOOR = 18

/**
 * Compress low availability into a stable floor while preserving a uniform
 * visual scale for the useful 85%-100% range.
 */
export function availabilityBarHeight(
  successRate: number,
  floor = AVAILABILITY_BAR_HEIGHT_FLOOR
): number {
  const safeFloor = Math.min(100, Math.max(0, floor))
  const safeRate = Number.isFinite(successRate)
    ? Math.min(100, Math.max(0, successRate))
    : 0
  if (safeRate <= 85) return safeFloor
  return safeFloor + ((safeRate - 85) / 15) * (100 - safeFloor)
}

export function availabilityBarClass(successRate: number): string {
  if (successRate >= HEALTHY_AVAILABILITY_THRESHOLD) {
    return 'bg-emerald-500/90 dark:bg-emerald-400/85'
  }
  if (successRate >= UNSTABLE_AVAILABILITY_THRESHOLD) {
    return 'bg-amber-400/90 dark:bg-amber-300/80'
  }
  return 'bg-rose-500/90 dark:bg-rose-400/85'
}
