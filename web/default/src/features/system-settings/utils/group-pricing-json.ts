/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import { safeJsonParseWithValidation } from './json-parser'
import { isObjectRecord, isStringArray } from './json-validators'

const isGroupIconTypeMap = (data: unknown): data is Record<string, number> =>
  isObjectRecord(data) &&
  Object.values(data).every(
    (value) =>
      typeof value === 'number' && Number.isInteger(value) && value >= 0
  )

export function parseGroupOrder(value: string | undefined | null): string[] {
  return safeJsonParseWithValidation<string[]>(value, {
    fallback: [],
    validator: isStringArray,
    context: 'group order',
  })
}

export function parseGroupIconTypes(
  value: string | undefined | null
): Record<string, number> {
  return safeJsonParseWithValidation<Record<string, number>>(value, {
    fallback: {},
    validator: isGroupIconTypeMap,
    context: 'group icon types',
  })
}
