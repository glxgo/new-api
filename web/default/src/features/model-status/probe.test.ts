import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import type { GroupProbeSummary } from '@/features/performance-metrics/types'
import {
  formatProbeTime,
  probeErrorLabel,
  probeLabel,
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

  test('warns when a multi-channel group has only one healthy channel left', () => {
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

  test('keeps unconfirmed all-channel failures in the warning state', () => {
    const probe = probeSummary({
      status: 'degraded',
      total_channels: 2,
      checked_channels: 2,
      degraded_channels: 2,
    })
    const tone = probeTone(probe)
    assert.equal(tone, 'degraded')
    assert.equal(probeLabel(tone, probe), '状态确认中')
  })

  test('formats public labels without exposing channel details', () => {
    assert.equal(probeLabel('degraded'), '状态确认中')
    assert.equal(probeErrorLabel('authentication'), '鉴权失败')
    assert.equal(formatProbeTime(1_000, 1_061_000), '1 分钟前')
  })
})
