import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { availabilityBarClass } from './visuals.ts'

describe('availabilityBarClass', () => {
  test('uses green at or above 90 percent', () => {
    assert.match(availabilityBarClass(100), /emerald/)
    assert.match(availabilityBarClass(90), /emerald/)
  })

  test('uses yellow from 80 percent up to but excluding 90 percent', () => {
    assert.match(availabilityBarClass(89.9), /amber/)
    assert.match(availabilityBarClass(80), /amber/)
  })

  test('uses red below 80 percent', () => {
    assert.match(availabilityBarClass(79.9), /rose/)
    assert.match(availabilityBarClass(0), /rose/)
  })
})
