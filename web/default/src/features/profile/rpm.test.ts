import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { resolveCurrentRpm, resolveRpmLimit } from './rpm.ts'

describe('resolveRpmLimit', () => {
  test('uses the backend limit when the new field is available', () => {
    assert.equal(resolveRpmLimit({ concurrency_limit: 8, rpm_limit: 20 }), 20)
  })

  test('derives the real limit from concurrency for an older backend', () => {
    assert.equal(resolveRpmLimit({ concurrency_limit: 1000 }), 1500)
    assert.equal(resolveRpmLimit({ concurrency_limit: 9 }), 14)
  })
})

describe('resolveCurrentRpm', () => {
  test('does not present a missing runtime counter as zero', () => {
    assert.equal(resolveCurrentRpm({ concurrency_limit: 1000 }), null)
    assert.equal(resolveCurrentRpm({ current_rpm: 0 }), 0)
  })
})
