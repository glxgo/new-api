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
  DEFAULT_CURRENCY_CONFIG,
  mergePersistedSystemConfigState,
  migratePersistedSystemConfigState,
  normalizeCurrencyConfig,
} from './system-config-persistence.ts'

describe('system config persistence recovery', () => {
  test('fills every currency default when an old cache has no currency', () => {
    assert.deepEqual(
      normalizeCurrencyConfig(undefined),
      DEFAULT_CURRENCY_CONFIG
    )
    assert.deepEqual(
      migratePersistedSystemConfigState({
        config: { systemName: 'Legacy', logo: '/legacy.png' },
      }),
      {
        config: {
          systemName: 'Legacy',
          logo: '/legacy.png',
          currency: DEFAULT_CURRENCY_CONFIG,
        },
      }
    )
  })

  test('replaces invalid persisted currency fields with safe defaults', () => {
    assert.deepEqual(
      normalizeCurrencyConfig({
        displayInCurrency: 'yes',
        quotaDisplayType: 'BROKEN',
        quotaPerUnit: 0,
        usdExchangeRate: Number.NaN,
        customCurrencySymbol: null,
        customCurrencyExchangeRate: Number.POSITIVE_INFINITY,
      }),
      DEFAULT_CURRENCY_CONFIG
    )
  })

  test('deep-merges a partial persisted config without replacing live actions', () => {
    const setLoading = () => undefined
    const currentState = {
      config: {
        systemName: 'Default',
        logo: '/default.png',
        currency: DEFAULT_CURRENCY_CONFIG,
      },
      loading: true,
      loadedLogoUrl: '/default.png',
      setLoading,
    }

    const merged = mergePersistedSystemConfigState(
      {
        config: {
          systemName: 'Persisted',
          currency: { quotaDisplayType: 'CNY', usdExchangeRate: 7.2 },
        },
        loadedLogoUrl: '/persisted.png',
        setLoading: 'corrupted',
      },
      currentState
    )

    assert.equal(merged.setLoading, setLoading)
    assert.equal(merged.config.systemName, 'Default')
    assert.equal(merged.config.logo, '/default.png')
    assert.deepEqual(merged.config.currency, {
      ...DEFAULT_CURRENCY_CONFIG,
      quotaDisplayType: 'CNY',
      usdExchangeRate: 7.2,
    })
    assert.equal(merged.loadedLogoUrl, '/default.png')
  })

  test('does not restore legacy or remote branding from persisted state', () => {
    const currentState = {
      config: {
        systemName: '飓星API',
        logo: '/logo.png?v=bundled',
        currency: DEFAULT_CURRENCY_CONFIG,
      },
      loadedLogoUrl: '/logo.png?v=bundled',
    }

    const merged = mergePersistedSystemConfigState(
      {
        config: {
          systemName: 'New API',
          logo: 'https://example.com/old-logo.png',
        },
        loadedLogoUrl: 'https://example.com/old-logo.png',
      },
      currentState
    )

    assert.equal(merged.config.systemName, '飓星API')
    assert.equal(merged.config.logo, '/logo.png?v=bundled')
    assert.equal(merged.loadedLogoUrl, '/logo.png?v=bundled')
  })
})
