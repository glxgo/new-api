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
import { describe, test } from 'node:test'
import {
  hasCompleteHealthMetrics,
  resolveModelStatusGroups,
  summarizeAvailabilitySeries,
} from './compat.ts'

describe('resolveModelStatusGroups', () => {
  test('keeps groups returned by the deployed legacy endpoint', () => {
    const groups = resolveModelStatusGroups({
      groups: [
        {
          group: '套餐专用分组',
          cache_rate: 95.8,
          request_count: 6407,
          cache_tokens: 1,
          prompt_tokens: 1,
        },
      ],
    })

    assert.deepEqual(groups, ['套餐专用分组'])
  })

  test('merges new endpoint groups and pricing groups without duplicates', () => {
    const groups = resolveModelStatusGroups(
      {
        available_groups: ['default', '套餐专用分组'],
        groups: [],
      },
      ['套餐专用分组', 'vip']
    )

    assert.deepEqual(groups, ['default', '套餐专用分组', 'vip'])
  })

  test('never exposes the internal auto routing group as a status card', () => {
    const groups = resolveModelStatusGroups(
      {
        available_groups: ['auto', 'default'],
        groups: [
          {
            group: 'auto',
            cache_rate: 0,
            request_count: 1,
            cache_tokens: 0,
            prompt_tokens: 0,
          },
        ],
      },
      ['auto', '套餐专用分组']
    )

    assert.deepEqual(groups, ['default', '套餐专用分组'])
  })
})

describe('hasCompleteHealthMetrics', () => {
  test('distinguishes legacy cache-only summaries from full health data', () => {
    assert.equal(
      hasCompleteHealthMetrics({
        group: 'legacy',
        cache_rate: 80,
        request_count: 10,
        cache_tokens: 8,
        prompt_tokens: 10,
      }),
      false
    )
    assert.equal(
      hasCompleteHealthMetrics({
        group: 'new',
        cache_rate: 80,
        request_count: 10,
        cache_tokens: 8,
        prompt_tokens: 10,
        success_rate: 99,
      }),
      true
    )
  })
})

describe('summarizeAvailabilitySeries', () => {
  test('groups a seven-day window into at most 24 weighted segments', () => {
    const series = Array.from({ length: 28 }, (_, index) => ({
      ts: index * 6 * 3600,
      request_count: index === 0 ? 9 : 1,
      success_count: index === 0 ? 0 : 1,
      avg_ttft_ms: 0,
      avg_latency_ms: 0,
      success_rate: index === 0 ? 0 : 100,
      avg_tps: 0,
      cache_rate: 0,
    }))

    const result = summarizeAvailabilitySeries(series, 168)

    assert.ok(result.length <= 24)
    assert.equal(result[0]?.successRate, 10)
  })

  test('falls back to averaging reported rates when request counts are absent', () => {
    const result = summarizeAvailabilitySeries(
      [
        {
          ts: 0,
          avg_ttft_ms: 0,
          avg_latency_ms: 0,
          success_rate: 90,
          avg_tps: 0,
          cache_rate: 0,
        },
        {
          ts: 1800,
          avg_ttft_ms: 0,
          avg_latency_ms: 0,
          success_rate: 100,
          avg_tps: 0,
          cache_rate: 0,
        },
      ],
      24
    )

    assert.equal(result[0]?.successRate, 95)
  })
})
