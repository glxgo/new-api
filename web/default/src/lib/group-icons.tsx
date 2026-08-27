import {
  getChannelTypeIcon,
  getChannelTypeLabel,
} from '@/features/channels/lib/channel-utils'
import { getLobeIcon } from './lobe-icon'

export function getGroupIconNode(iconType?: number, size = 20) {
  if (!Number.isFinite(iconType) || !iconType || iconType < 0) return null
  return getLobeIcon(`${getChannelTypeIcon(Number(iconType))}.Color`, size)
}

export function getGroupIconLabel(iconType?: number) {
  if (!Number.isFinite(iconType) || !iconType || iconType < 0) return ''
  return getChannelTypeLabel(Number(iconType))
}
