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
/**
 * LobeHub Icon Loader
 * Resolve and render the provider icons used by this application without
 * importing the package-wide icon barrel.
 *
 * Supports:
 * - Basic: "OpenAI", "OpenAI.Color"
 * - Chained properties: "OpenAI.Avatar.type={'platform'}"
 * - Size parameter: getLobeIcon("OpenAI", 20)
 */
import Ai360 from '@lobehub/icons/es/Ai360'
import Aws from '@lobehub/icons/es/Aws'
import Azure from '@lobehub/icons/es/Azure'
import AzureAI from '@lobehub/icons/es/AzureAI'
import Baidu from '@lobehub/icons/es/Baidu'
import Claude from '@lobehub/icons/es/Claude'
import Cloudflare from '@lobehub/icons/es/Cloudflare'
import Cohere from '@lobehub/icons/es/Cohere'
import Coze from '@lobehub/icons/es/Coze'
import DeepSeek from '@lobehub/icons/es/DeepSeek'
import Dify from '@lobehub/icons/es/Dify'
import Doubao from '@lobehub/icons/es/Doubao'
import FastGPT from '@lobehub/icons/es/FastGPT'
import Gemini from '@lobehub/icons/es/Gemini'
import GithubCopilot from '@lobehub/icons/es/GithubCopilot'
import Google from '@lobehub/icons/es/Google'
import Grok from '@lobehub/icons/es/Grok'
import Hunyuan from '@lobehub/icons/es/Hunyuan'
import Jimeng from '@lobehub/icons/es/Jimeng'
import Jina from '@lobehub/icons/es/Jina'
import Kling from '@lobehub/icons/es/Kling'
import Meta from '@lobehub/icons/es/Meta'
import Midjourney from '@lobehub/icons/es/Midjourney'
import Minimax from '@lobehub/icons/es/Minimax'
import Mistral from '@lobehub/icons/es/Mistral'
import Moonshot from '@lobehub/icons/es/Moonshot'
import Ollama from '@lobehub/icons/es/Ollama'
import OpenAI from '@lobehub/icons/es/OpenAI'
import OpenCode from '@lobehub/icons/es/OpenCode'
import OpenRouter from '@lobehub/icons/es/OpenRouter'
import Perplexity from '@lobehub/icons/es/Perplexity'
import Qwen from '@lobehub/icons/es/Qwen'
import Replicate from '@lobehub/icons/es/Replicate'
import SiliconCloud from '@lobehub/icons/es/SiliconCloud'
import Spark from '@lobehub/icons/es/Spark'
import Suno from '@lobehub/icons/es/Suno'
import Venice from '@lobehub/icons/es/Venice'
import Vidu from '@lobehub/icons/es/Vidu'
import Volcengine from '@lobehub/icons/es/Volcengine'
import Wenxin from '@lobehub/icons/es/Wenxin'
import XAI from '@lobehub/icons/es/XAI'
import Xinference from '@lobehub/icons/es/Xinference'
import Yi from '@lobehub/icons/es/Yi'
import Zhipu from '@lobehub/icons/es/Zhipu'

const LOBE_ICONS = {
  Ai360,
  Aws,
  Azure,
  AzureAI,
  Baidu,
  Claude,
  Cloudflare,
  Cohere,
  Coze,
  DeepSeek,
  Dify,
  Doubao,
  FastGPT,
  Gemini,
  GithubCopilot,
  Google,
  Grok,
  Hunyuan,
  Jina,
  Jimeng,
  Kling,
  Meta,
  Midjourney,
  Minimax,
  Mistral,
  Moonshot,
  Ollama,
  OpenAI,
  OpenCode,
  OpenRouter,
  Perplexity,
  Qwen,
  Replicate,
  SiliconCloud,
  Spark,
  Suno,
  Venice,
  Vidu,
  Volcengine,
  Wenxin,
  XAI,
  Xinference,
  Yi,
  Zhipu,
} as const

const LOBE_ICON_ALIASES: Record<string, keyof typeof LOBE_ICONS> = {
  anthropic: 'Claude',
  minimax: 'Minimax',
  veniceai: 'Venice',
}

function resolveBaseIcon(baseKey: string): unknown {
  const exact = LOBE_ICONS[baseKey as keyof typeof LOBE_ICONS]
  if (exact) return exact

  const canonicalKey =
    LOBE_ICON_ALIASES[baseKey.toLowerCase()] ??
    (Object.keys(LOBE_ICONS).find(
      (key) => key.toLowerCase() === baseKey.toLowerCase()
    ) as keyof typeof LOBE_ICONS | undefined)

  return canonicalKey ? LOBE_ICONS[canonicalKey] : undefined
}

/**
 * Parse a property value from string to appropriate type
 * @param raw - Raw string value
 * @returns Parsed value (boolean, number, or string)
 */
function parseValue(raw: string | undefined | null): string | number | boolean {
  if (raw == null) return true

  let v = String(raw).trim()

  // Remove curly braces
  if (v.startsWith('{') && v.endsWith('}')) {
    v = v.slice(1, -1).trim()
  }

  // Remove quotes
  if (
    (v.startsWith('"') && v.endsWith('"')) ||
    (v.startsWith("'") && v.endsWith("'"))
  ) {
    return v.slice(1, -1)
  }

  // Boolean
  if (v === 'true') return true
  if (v === 'false') return false

  // Number
  if (/^-?\d+(?:\.\d+)?$/.test(v)) return Number(v)

  // Return as string
  return v
}

/**
 * Get LobeHub icon component by name
 * @param iconName - Icon name/description (e.g., "OpenAI", "OpenAI.Color", "Claude.Avatar")
 * @param size - Icon size (default: 20)
 * @returns Icon component or fallback
 *
 * @example
 * getLobeIcon("OpenAI", 24)
 * getLobeIcon("OpenAI.Color", 20)
 * getLobeIcon("Claude.Avatar.type={'platform'}", 32)
 */
export function getLobeIcon(
  iconName: string | undefined | null,
  size: number = 20
): React.ReactNode {
  if (!iconName || typeof iconName !== 'string') {
    return (
      <div
        className='bg-muted text-muted-foreground flex items-center justify-center rounded-full text-xs font-medium'
        style={{ width: size, height: size }}
      >
        ?
      </div>
    )
  }

  const trimmedName = iconName.trim()
  if (!trimmedName) {
    return (
      <div
        className='bg-muted text-muted-foreground flex items-center justify-center rounded-full text-xs font-medium'
        style={{ width: size, height: size }}
      >
        ?
      </div>
    )
  }

  // Parse component path and chained properties
  const segments = trimmedName.split('.')
  const baseKey = segments[0]
  const BaseIcon = resolveBaseIcon(baseKey) as
    | Record<string, unknown>
    | undefined

  let IconComponent: React.ComponentType<Record<string, unknown>> | undefined
  let propStartIndex: number

  if (BaseIcon && segments.length > 1 && BaseIcon[segments[1]]) {
    IconComponent = BaseIcon[segments[1]] as React.ComponentType<
      Record<string, unknown>
    >
    propStartIndex = 2
  } else {
    IconComponent = BaseIcon as
      | React.ComponentType<Record<string, unknown>>
      | undefined
    propStartIndex = segments.length > 1 && /^[A-Z]/.test(segments[1]) ? 2 : 1
  }

  // Fallback if icon not found
  if (
    !IconComponent ||
    (typeof IconComponent !== 'function' && typeof IconComponent !== 'object')
  ) {
    const firstLetter = trimmedName.charAt(0).toUpperCase()
    return (
      <div
        className='bg-muted text-muted-foreground flex items-center justify-center rounded-full text-xs font-medium'
        style={{ width: size, height: size }}
      >
        {firstLetter}
      </div>
    )
  }

  // Parse chained properties (e.g., "type={'platform'}", "shape='square'")
  const props: Record<string, string | number | boolean> = {}

  for (let i = propStartIndex; i < segments.length; i++) {
    const seg = segments[i]
    if (!seg) continue

    const eqIdx = seg.indexOf('=')
    if (eqIdx === -1) {
      props[seg.trim()] = true
      continue
    }

    const key = seg.slice(0, eqIdx).trim()
    const valRaw = seg.slice(eqIdx + 1).trim()
    props[key] = parseValue(valRaw)
  }

  // Set size if not explicitly specified in the string
  if (props.size == null && size != null) {
    props.size = size
  }

  return <IconComponent {...props} />
}

/**
 * getLobeIconWithFallback 按候选图标名顺序, 返回第一个真实存在于 LobeIcons 的图标;
 * 全部缺失时退回 getLobeIcon 的首字母占位。供排行榜等多候选场景使用。
 */
export function getLobeIconWithFallback(
  iconNames: (string | undefined | null)[],
  size: number = 20
): React.ReactNode {
  for (const name of iconNames) {
    if (!name || !name.trim()) continue
    const baseKey = name.trim().split('.')[0]
    if (resolveBaseIcon(baseKey)) {
      return getLobeIcon(name, size)
    }
  }
  const firstNonEmpty = iconNames.find((n) => n && n.trim())
  return getLobeIcon(firstNonEmpty ?? null, size)
}
