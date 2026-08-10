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
import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { DEFAULT_SYSTEM_NAME, DEFAULT_LOGO } from '@/lib/constants'
import {
  DEFAULT_CURRENCY_CONFIG,
  SYSTEM_CONFIG_STORAGE_VERSION,
  mergePersistedSystemConfigState,
  migratePersistedSystemConfigState,
  normalizeCurrencyConfig,
  type SystemConfig,
} from './system-config-persistence'

export {
  DEFAULT_CURRENCY_CONFIG,
  normalizeCurrencyConfig,
} from './system-config-persistence'
export type {
  CurrencyConfig,
  CurrencyDisplayType,
  SystemConfig,
} from './system-config-persistence'

interface SystemConfigState {
  config: SystemConfig
  loading: boolean
  loadedLogoUrl: string
  setConfig: (config: Partial<SystemConfig>) => void
  setLoadedLogoUrl: (url: string) => void
  setLoading: (loading: boolean) => void
}

/**
 * System configuration store with automatic persistence
 * Manages system name, logo, footer HTML and loading states
 */
export const useSystemConfigStore = create<SystemConfigState>()(
  persist(
    (set) => ({
      config: {
        systemName: DEFAULT_SYSTEM_NAME,
        logo: DEFAULT_LOGO,
        currency: { ...DEFAULT_CURRENCY_CONFIG },
      },
      loading: true,
      loadedLogoUrl: DEFAULT_LOGO,
      setConfig: (newConfig) =>
        set((state) => ({
          config: {
            ...state.config,
            ...newConfig,
            currency: normalizeCurrencyConfig({
              ...state.config.currency,
              ...(newConfig.currency ?? {}),
            }),
          },
        })),
      setLoadedLogoUrl: (url) => set({ loadedLogoUrl: url }),
      setLoading: (loading) => set({ loading }),
    }),
    {
      name: 'system-config-storage',
      version: SYSTEM_CONFIG_STORAGE_VERSION,
      migrate: (persistedState) =>
        migratePersistedSystemConfigState(persistedState),
      merge: (persistedState, currentState) =>
        mergePersistedSystemConfigState(persistedState, currentState),
      partialize: (state) => ({
        config: state.config,
        loadedLogoUrl: state.loadedLogoUrl,
      }),
    }
  )
)

// Selector helpers for convenience
export const getSystemName = () =>
  useSystemConfigStore.getState().config.systemName

export const getLogo = () => useSystemConfigStore.getState().config.logo

export const getFooterHtml = () =>
  useSystemConfigStore.getState().config.footerHtml
