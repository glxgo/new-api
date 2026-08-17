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
import { readFileSync } from 'node:fs'
import test from 'node:test'

const readSource = (relativePath: string) =>
  readFileSync(new URL(relativePath, import.meta.url), 'utf8')

test('subscription and virtual membership quota values are labelled as remaining', () => {
  const subscriptionSources = [
    readSource('./my-subscriptions-detail.tsx'),
    readSource('./subscription-plans-card.tsx'),
  ]
  const membershipSources = [
    readSource('./my-virtual-memberships-detail.tsx'),
    readSource('../../virtual-membership/index.tsx'),
  ]

  for (const source of subscriptionSources) {
    assert.match(source, /\{t\('Remaining'\)\}/)
  }
  for (const source of membershipSources) {
    assert.match(source, /剩余\s+/)
  }
})
