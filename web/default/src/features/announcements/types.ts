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
export interface UserAnnouncement {
  id: number
  title?: string
  content: string
  publishDate?: string
  type?: 'default' | 'ongoing' | 'success' | 'warning' | 'error'
  extra?: string
  unread: boolean
}

export function getAnnouncementTitle(item: UserAnnouncement): string {
  if (item.title?.trim()) return item.title.trim()
  const firstLine = item.content
    .split('\n')
    .map((line) => line.replace(/^\s*[#>*\-\d.]+\s*/, '').trim())
    .find(Boolean)
  return firstLine?.slice(0, 80) || '公告'
}

export interface UserAnnouncementData {
  enabled: boolean
  announcements: UserAnnouncement[]
  unread: UserAnnouncement[]
}
