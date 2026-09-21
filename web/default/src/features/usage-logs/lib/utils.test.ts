import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  formatCapacitySnapshot,
  getDefaultCommonLogTimeRange,
  getDefaultTimeRange,
} from './utils.ts'

test('capacity snapshots retain unlimited counts without inventing historical data', () => {
  assert.equal(formatCapacitySnapshot(1, 0), '1/\u221e')
  assert.equal(formatCapacitySnapshot(23, 0), '23/\u221e')
  assert.equal(formatCapacitySnapshot(2, 200), '2/200')
  assert.equal(formatCapacitySnapshot(0, 200), '0/200')
  assert.equal(formatCapacitySnapshot(0, 0), null)
})

for (const getRange of [getDefaultTimeRange, getDefaultCommonLogTimeRange])
  describe(getRange.name, () => {
    test('returns today at midnight through one hour in the future', () => {
      const before = new Date()
      const { start, end } = getRange()
      const after = new Date()

      const expectedStart = new Date(before)
      expectedStart.setHours(0, 0, 0, 0)
      assert.equal(start.getTime(), expectedStart.getTime())

      assert.ok(end.getTime() >= before.getTime() + 60 * 60 * 1000)
      assert.ok(end.getTime() <= after.getTime() + 60 * 60 * 1000)
    })
  })
