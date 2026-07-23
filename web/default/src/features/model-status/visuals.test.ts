import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { availabilityBarClass } from './visuals.ts'

describe('availabilityBarClass', () => {
  test('uses green at or above 96 percent', () => {
    assert.match(availabilityBarClass(100), /emerald/)
    assert.match(availabilityBarClass(96), /emerald/)
  })

  test('uses yellow from 90 percent up to but excluding 96 percent', () => {
    assert.match(availabilityBarClass(95.9), /amber/)
    assert.match(availabilityBarClass(90), /amber/)
  })

  test('uses red below 90 percent', () => {
    assert.match(availabilityBarClass(89.9), /rose/)
    assert.match(availabilityBarClass(0), /rose/)
  })
})
