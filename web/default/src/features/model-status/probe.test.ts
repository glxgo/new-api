import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  formatProbeTime,
  probeErrorLabel,
  probeLabel,
  probeTone,
} from './probe.ts'

describe('model status probe helpers', () => {
  test('does not invent health before a channel has been checked', () => {
    assert.equal(probeTone(undefined), 'unknown')
    assert.equal(
      probeTone({
        status: 'healthy',
        total_channels: 2,
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
      }),
      'unknown'
    )
  })

  test('formats public labels without exposing channel details', () => {
    assert.equal(probeLabel('degraded'), '部分波动')
    assert.equal(probeErrorLabel('authentication'), '鉴权失败')
    assert.equal(formatProbeTime(1_000, 1_061_000), '1 分钟前')
  })
})
