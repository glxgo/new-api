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
import i18n, { type BackendModule, type ReadCallback } from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'
import {
  normalizeInterfaceLanguage,
  type InterfaceLanguageCode,
} from './languages'

const languageLoaders = {
  en: () => import('./locales/en.json'),
  zh: () => import('./locales/zh.json'),
  fr: () => import('./locales/fr.json'),
  ru: () => import('./locales/ru.json'),
  ja: () => import('./locales/ja.json'),
  vi: () => import('./locales/vi.json'),
} satisfies Record<InterfaceLanguageCode, () => Promise<unknown>>

const lazyTranslationsBackend: BackendModule = {
  type: 'backend',
  init: () => undefined,
  read(language: string, _namespace: string, callback: ReadCallback) {
    const normalized = normalizeInterfaceLanguage(
      language
    ) as InterfaceLanguageCode

    languageLoaders[normalized]()
      .then((module) => {
        const resources = (
          module as {
            default: { translation: Record<string, unknown> }
          }
        ).default
        callback(null, resources.translation)
      })
      .catch((error: unknown) => {
        callback(
          error instanceof Error ? error : new Error(String(error)),
          null
        )
      })
  },
}

export const i18nReady = i18n
  .use(LanguageDetector)
  .use(lazyTranslationsBackend)
  .use(initReactI18next)
  .init({
    // Keep an explicit user selection, but do not let the browser/OS locale
    // silently turn a first visit into English. New users default to Chinese.
    fallbackLng: 'zh',
    supportedLngs: ['en', 'zh', 'fr', 'ru', 'ja', 'vi'],
    load: 'languageOnly', // Convert zh-CN -> zh
    nsSeparator: false, // Allow literal colons in keys (e.g., URLs, labels)
    debug: import.meta.env.DEV,
    interpolation: {
      escapeValue: false, // not needed for react as it escapes by default
    },
    detection: {
      order: ['localStorage'],
      lookupLocalStorage: 'i18nextLng',
      caches: ['localStorage'],
    },
  })

export default i18n
