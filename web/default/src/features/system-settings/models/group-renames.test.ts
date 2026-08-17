import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { detectSingleGroupRename } from './group-renames.ts'

describe('detectSingleGroupRename', () => {
  test('returns an explicit old-to-new mapping for one renamed group', () => {
    assert.deepEqual(
      detectSingleGroupRename(
        '{"default":1,"old":0.5}',
        '{"default":1,"new":0.5}'
      ),
      { old: 'new' }
    )
  })

  test('does not guess when a save contains multiple additions or deletions', () => {
    assert.deepEqual(
      detectSingleGroupRename('{"a":1,"b":1}', '{"c":1,"d":1}'),
      {}
    )
  })
})
