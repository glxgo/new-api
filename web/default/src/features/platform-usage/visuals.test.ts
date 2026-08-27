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
import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  getPlatformAccountPage,
  metricCardSurfaceClass,
  PLATFORM_ACCOUNT_PAGE_SIZE,
} from './visuals.ts'

const account = (code: string) => ({
  code,
  plan_type: 'pro',
  available: true,
  enabled: true,
  windows: [],
})

test('the highlighted usage card keeps a pure white light-mode surface', () => {
  const className = metricCardSurfaceClass(true)

  assert.match(className, /(?:^|\s)bg-white(?:\s|$)/)
  assert.doesNotMatch(className, /gradient|rgba\(16,185,129/)
})

test('the public account pool paginates eight accounts and clamps the page', () => {
  const accounts = Array.from(
    { length: PLATFORM_ACCOUNT_PAGE_SIZE + 1 },
    (_, i) => account(`A-${i}`)
  )

  const first = getPlatformAccountPage(accounts, 1)
  assert.equal(first.totalPages, 2)
  assert.equal(first.currentPage, 1)
  assert.deepEqual(
    first.accounts.map((item) => item.code),
    ['A-0', 'A-1', 'A-2', 'A-3', 'A-4', 'A-5', 'A-6', 'A-7']
  )

  const clamped = getPlatformAccountPage(accounts, 99)
  assert.equal(clamped.currentPage, 2)
  assert.deepEqual(
    clamped.accounts.map((item) => item.code),
    ['A-8']
  )
})
