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
import type { GroupProbeSummary } from '@/features/performance-metrics/types'
import {
  formatProbeTime,
  probeErrorLabel,
  probeLabel,
  probeSeriesHealthyPercentage,
  probeSeriesTone,
  probeTone,
} from './probe.ts'

function probeSummary(
  overrides: Partial<GroupProbeSummary>
): GroupProbeSummary {
  return {
    status: 'unknown',
    total_channels: 0,
    checked_channels: 0,
    healthy_channels: 0,
    degraded_channels: 0,
    unhealthy_channels: 0,
    success_rate: 0,
    avg_latency_ms: 0,
    avg_ttft_ms: 0,
    last_probe_ts: 0,
    last_success_ts: 0,
    series: [],
    ...overrides,
  }
}

describe('model status probe helpers', () => {
  test('does not invent health before a channel has been checked', () => {
    assert.equal(probeTone(undefined), 'unknown')
    assert.equal(
      probeTone(
        probeSummary({
          status: 'healthy',
          total_channels: 2,
          checked_channels: 0,
        })
      ),
      'unknown'
    )
  })

  test('keeps a partially healthy pool green while at least two channels remain', () => {
    const probe = probeSummary({
      status: 'degraded',
      total_channels: 3,
      checked_channels: 3,
      healthy_channels: 2,
      unhealthy_channels: 1,
    })
    const tone = probeTone(probe)
    assert.equal(tone, 'healthy')
    assert.equal(probeLabel(tone, probe), '部分正常')
  })

  test('marks a group yellow when fewer than half of its channels are healthy', () => {
    const probe = probeSummary({
      status: 'degraded',
      total_channels: 3,
      checked_channels: 3,
      healthy_channels: 1,
      unhealthy_channels: 2,
    })
    const tone = probeTone(probe)
    assert.equal(tone, 'degraded')
    assert.equal(probeLabel(tone, probe), '部分正常')
  })

  test('colors and sizes historical bars from channel health instead of request success rate', () => {
    const greenPoint = {
      ts: 1,
      probe_count: 8,
      success_count: 1,
      success_rate: 12.5,
      avg_latency_ms: 0,
      avg_ttft_ms: 0,
      total_channels: 4,
      checked_channels: 4,
      healthy_channels: 2,
    }
    const yellowPoint = {
      ...greenPoint,
      healthy_channels: 1,
      success_rate: 100,
    }
    const redPoint = { ...greenPoint, healthy_channels: 0 }

    assert.equal(probeSeriesHealthyPercentage(greenPoint), 50)
    assert.equal(probeSeriesTone(greenPoint), 'healthy')
    assert.equal(probeSeriesHealthyPercentage(yellowPoint), 25)
    assert.equal(probeSeriesTone(yellowPoint), 'degraded')
    assert.equal(probeSeriesHealthyPercentage(redPoint), 0)
    assert.equal(probeSeriesTone(redPoint), 'unhealthy')
  })

  test('keeps a healthy single-channel group green', () => {
    const probe = probeSummary({
      status: 'healthy',
      total_channels: 1,
      checked_channels: 1,
      healthy_channels: 1,
    })
    const tone = probeTone(probe)
    assert.equal(tone, 'healthy')
    assert.equal(probeLabel(tone, probe), '全部正常')
  })

  test('marks only confirmed all-channel failure as a service error', () => {
    const probe = probeSummary({
      status: 'unhealthy',
      total_channels: 2,
      checked_channels: 2,
      unhealthy_channels: 2,
    })
    const tone = probeTone(probe)
    assert.equal(tone, 'unhealthy')
    assert.equal(probeLabel(tone, probe), '服务异常')
  })

  test('marks the group red when every checked channel is unavailable', () => {
    const probe = probeSummary({
      status: 'degraded',
      total_channels: 2,
      checked_channels: 2,
      degraded_channels: 2,
    })
    const tone = probeTone(probe)
    assert.equal(tone, 'unhealthy')
    assert.equal(probeLabel(tone, probe), '服务异常')
  })

  test('formats public labels without exposing channel details', () => {
    assert.equal(probeLabel('degraded'), '状态确认中')
    assert.equal(probeErrorLabel('authentication'), '鉴权失败')
    assert.equal(formatProbeTime(1_000, 1_061_000), '1 分钟前')
  })
})
