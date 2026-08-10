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
export type CurrencyDisplayType = 'USD' | 'CNY' | 'TOKENS' | 'CUSTOM'

export interface CurrencyConfig {
  displayInCurrency: boolean
  quotaDisplayType: CurrencyDisplayType
  quotaPerUnit: number
  usdExchangeRate: number
  customCurrencySymbol: string
  customCurrencyExchangeRate: number
}

export interface SystemConfig {
  systemName: string
  logo: string
  footerHtml?: string
  demoSiteEnabled?: boolean
  displayTokenStatEnabled?: boolean
  currency: CurrencyConfig
}

export const DEFAULT_CURRENCY_CONFIG: CurrencyConfig = {
  displayInCurrency: true,
  quotaDisplayType: 'USD',
  quotaPerUnit: 500000,
  usdExchangeRate: 1,
  customCurrencySymbol: '¤',
  customCurrencyExchangeRate: 1,
}

export const SYSTEM_CONFIG_STORAGE_VERSION = 1

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function positiveFiniteNumber(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
    ? value
    : fallback
}

function isCurrencyDisplayType(value: unknown): value is CurrencyDisplayType {
  return (
    value === 'USD' ||
    value === 'CNY' ||
    value === 'TOKENS' ||
    value === 'CUSTOM'
  )
}

export function normalizeCurrencyConfig(value: unknown): CurrencyConfig {
  const raw = isRecord(value) ? value : {}
  return {
    displayInCurrency:
      typeof raw.displayInCurrency === 'boolean'
        ? raw.displayInCurrency
        : DEFAULT_CURRENCY_CONFIG.displayInCurrency,
    quotaDisplayType: isCurrencyDisplayType(raw.quotaDisplayType)
      ? raw.quotaDisplayType
      : DEFAULT_CURRENCY_CONFIG.quotaDisplayType,
    quotaPerUnit: positiveFiniteNumber(
      raw.quotaPerUnit,
      DEFAULT_CURRENCY_CONFIG.quotaPerUnit
    ),
    usdExchangeRate: positiveFiniteNumber(
      raw.usdExchangeRate,
      DEFAULT_CURRENCY_CONFIG.usdExchangeRate
    ),
    customCurrencySymbol:
      typeof raw.customCurrencySymbol === 'string' &&
      raw.customCurrencySymbol.trim()
        ? raw.customCurrencySymbol
        : DEFAULT_CURRENCY_CONFIG.customCurrencySymbol,
    customCurrencyExchangeRate: positiveFiniteNumber(
      raw.customCurrencyExchangeRate,
      DEFAULT_CURRENCY_CONFIG.customCurrencyExchangeRate
    ),
  }
}

export function migratePersistedSystemConfigState(
  persistedState: unknown
): Record<string, unknown> {
  const persisted = isRecord(persistedState) ? persistedState : {}
  const config = isRecord(persisted.config) ? persisted.config : {}
  return {
    ...persisted,
    config: {
      ...config,
      currency: normalizeCurrencyConfig(config.currency),
    },
  }
}

type SystemConfigStateShape = {
  config: SystemConfig
  loadedLogoUrl: string
}

export function mergePersistedSystemConfigState<
  TState extends SystemConfigStateShape,
>(persistedState: unknown, currentState: TState): TState {
  const migrated = migratePersistedSystemConfigState(persistedState)
  const persistedConfig = isRecord(migrated.config) ? migrated.config : {}
  const loadedLogoUrl =
    typeof migrated.loadedLogoUrl === 'string'
      ? migrated.loadedLogoUrl
      : currentState.loadedLogoUrl

  return {
    ...currentState,
    config: {
      ...currentState.config,
      ...persistedConfig,
      // Keep the bundled product identity. Persisted branding may point to an
      // older site name or a remote logo and must never win during hydration.
      systemName: currentState.config.systemName,
      logo: currentState.config.logo,
      currency: normalizeCurrencyConfig(persistedConfig.currency),
    },
    loadedLogoUrl:
      loadedLogoUrl === currentState.config.logo
        ? loadedLogoUrl
        : currentState.loadedLogoUrl,
  }
}
