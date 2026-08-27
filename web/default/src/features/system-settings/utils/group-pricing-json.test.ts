import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { parseGroupIconTypes, parseGroupOrder } from './group-pricing-json.ts'

describe('group pricing JSON compatibility', () => {
  test('treats legacy null group order as an empty array', () => {
    assert.deepEqual(parseGroupOrder('null'), [])
  })

  test('rejects non-array group order values instead of returning null', () => {
    assert.deepEqual(parseGroupOrder('{"group":"first"}'), [])
  })

  test('treats legacy null icon metadata as an empty object', () => {
    assert.deepEqual(parseGroupIconTypes('null'), {})
  })

  test('keeps valid icon metadata', () => {
    assert.deepEqual(parseGroupIconTypes('{"openai":1}'), { openai: 1 })
  })
})
