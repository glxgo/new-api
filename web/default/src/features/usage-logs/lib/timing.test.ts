import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { resolveFirstTokenMs } from './timing.ts'

describe('resolveFirstTokenMs', () => {
  test('prefers the upstream-to-server timing for new logs', () => {
    assert.equal(resolveFirstTokenMs({ upstream_frt: 1250, frt: 4250 }), 1250)
  })

  test('falls back to the historical request-ingress timing', () => {
    assert.equal(resolveFirstTokenMs({ frt: 4250 }), 4250)
  })

  test('rejects missing or invalid timing values', () => {
    assert.equal(resolveFirstTokenMs(undefined), null)
    assert.equal(resolveFirstTokenMs({ upstream_frt: 0, frt: 4250 }), null)
  })
})
