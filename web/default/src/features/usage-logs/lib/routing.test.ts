/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  getUserVisibleModelMapping,
  getUserVisibleRequestRouting,
} from './routing.ts'

describe('getUserVisibleModelMapping', () => {
  test('returns only the request and actual model names needed by the owner UI', () => {
    assert.deepEqual(
      getUserVisibleModelMapping('gpt-5.6-luna', {
        is_model_mapped: true,
        upstream_model_name: 'gpt-5.6-terra',
        admin_info: { use_channel: [8, 31] },
      }),
      {
        requestModel: 'gpt-5.6-luna',
        actualModel: 'gpt-5.6-terra',
      }
    )
  })

  test('does not claim a mapping without the persisted mapping flag', () => {
    assert.equal(
      getUserVisibleModelMapping('gpt-5.6-luna', {
        upstream_model_name: 'gpt-5.6-terra',
      }),
      null
    )
  })
})

describe('getUserVisibleRequestRouting', () => {
  test('returns the safe request route and conversion chain for a user log', () => {
    assert.deepEqual(
      getUserVisibleRequestRouting(2, {
        request_path: ' /v1/responses ',
        request_conversion: ['OpenAI Responses', 'Claude Messages'],
        admin_info: {
          use_channel: [8, 31],
          channel_affinity: { key_path: 'secret-path' },
        },
      }),
      {
        requestPath: '/v1/responses',
        conversionChain: ['OpenAI Responses', 'Claude Messages'],
      }
    )
  })

  test('does not create route sections for financial or refund logs', () => {
    const other = {
      request_path: '/v1/responses',
      request_conversion: ['OpenAI Responses'],
    }
    assert.equal(getUserVisibleRequestRouting(1, other), null)
    assert.equal(getUserVisibleRequestRouting(6, other), null)
  })
})
