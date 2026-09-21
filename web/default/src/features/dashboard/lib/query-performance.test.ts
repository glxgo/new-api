import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  buildDefaultDashboardFilters,
  getDashboardSnapshotRange,
} from './filters'

test('default dashboard queries use a shared 30-second boundary', () => {
  const filters = buildDefaultDashboardFilters()
  assert.equal(filters.end_timestamp!.getTime() % 30_000, 0)
  assert.equal(filters.start_timestamp!.getTime() % 30_000, 0)
})

test('visits within a snapshot reuse both bounds without changing the duration', () => {
  const first = getDashboardSnapshotRange(7, new Date('2026-09-20T10:01:03Z'))
  const second = getDashboardSnapshotRange(7, new Date('2026-09-20T10:01:28Z'))
  assert.deepEqual(first, second)
  assert.equal(first.end.getTime() - first.start.getTime(), 7 * 86400_000)
  const next = getDashboardSnapshotRange(7, new Date('2026-09-20T10:01:31Z'))
  assert.equal(next.end.getTime() - first.end.getTime(), 30_000)
})
