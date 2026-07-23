/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { LogOtherData } from '../types'

export function resolveFirstTokenMs(
  other: LogOtherData | null | undefined
): number | null {
  const value = other?.upstream_frt ?? other?.frt
  return value != null && Number.isFinite(value) && value > 0 ? value : null
}
