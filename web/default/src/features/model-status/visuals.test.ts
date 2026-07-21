import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { availabilityBarClass } from './visuals.ts'

describe('availabilityBarClass', () => {
  test('uses green for healthy data and yellow for every degraded state', () => {
    assert.match(availabilityBarClass(100), /emerald/)
    assert.match(availabilityBarClass(99), /emerald/)
    assert.match(availabilityBarClass(98.9), /amber/)
    assert.match(availabilityBarClass(0), /amber/)
  })
})
