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
export const SECURITY_SECTION_IDS = [
  'account-capacity',
  'rate-limit',
  'sensitive-words',
  'ssrf',
] as const

export type SecuritySectionId = (typeof SECURITY_SECTION_IDS)[number]
export const SECURITY_DEFAULT_SECTION: SecuritySectionId = 'account-capacity'
export const SECURITY_SECTION_TITLES = {
  'account-capacity': '账号容量',
  'rate-limit': 'Rate Limiting',
  'sensitive-words': 'Sensitive Words',
  ssrf: 'SSRF Protection',
} satisfies Record<SecuritySectionId, string>
