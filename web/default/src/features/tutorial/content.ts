export interface SiteGuide {
  id: 'site-guide'
  content: string
}

export type SiteGuidePlacement =
  | 'quick-start'
  | 'api-key'
  | 'model-selection'
  | 'client-import'
  | 'balance'
  | 'usage'
  | 'other'

export interface SiteGuideSection {
  id: string
  title: string
  content: string
  placement: SiteGuidePlacement
}

export type SiteGuideSectionsByPlacement = Record<
  SiteGuidePlacement,
  SiteGuideSection[]
>

export const DEFAULT_TUTORIAL_API_BASE_URL = 'https://token.stellaisle.com'

function normalizePublicBaseUrl(value?: string | null): string | null {
  const normalized = value?.trim().replace(/\/+$/, '') || ''
  if (!normalized) return null

  try {
    const url = new URL(normalized)
    const hostname = url.hostname.toLowerCase()
    if (
      hostname === 'localhost' ||
      hostname === '127.0.0.1' ||
      hostname === '::1'
    ) {
      return null
    }
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return null
    return normalized
  } catch {
    return null
  }
}

export function resolveTutorialApiBaseUrl(
  serverAddress?: string | null,
  browserOrigin?: string | null
): string {
  return (
    normalizePublicBaseUrl(serverAddress) ||
    normalizePublicBaseUrl(browserOrigin) ||
    DEFAULT_TUTORIAL_API_BASE_URL
  )
}

// 后台教程必须按原文完整保留。它可以没有任何 Markdown 标题，不能因为
// 页面导航需要分段而被过滤或改写。
export function getSiteGuide(content?: string | null): SiteGuide | null {
  const normalized = content?.trim() || ''
  if (!normalized) return null

  return {
    id: 'site-guide',
    content: normalized,
  }
}

function getSectionPlacement(title: string): SiteGuidePlacement {
  const ordinal = Number(title.match(/^\s*(\d+)/)?.[1])
  const ordinalPlacement: Record<number, SiteGuidePlacement> = {
    1: 'quick-start',
    2: 'balance',
    3: 'api-key',
    4: 'model-selection',
    5: 'client-import',
    6: 'client-import',
    7: 'usage',
  }
  if (ordinalPlacement[ordinal]) return ordinalPlacement[ordinal]

  if (/登录|注册|账号/.test(title)) return 'quick-start'
  if (/余额|套餐|充值/.test(title)) return 'balance'
  if (/API\s*Key|密钥/i.test(title)) return 'api-key'
  if (/模型/.test(title)) return 'model-selection'
  if (/客户端|CC\s*Switch|自动配置|端点/i.test(title)) {
    return 'client-import'
  }
  if (/用量|消耗|订单|日志/.test(title)) return 'usage'
  return 'other'
}

export function getSiteGuideSections(
  content?: string | null
): SiteGuideSection[] {
  const guide = getSiteGuide(content)
  if (!guide) return []

  const headings = Array.from(guide.content.matchAll(/^##\s+(.+?)\s*$/gm))
  if (headings.length === 0) {
    return [
      {
        id: 'site-guide-overview',
        title: '本站补充说明',
        content: guide.content,
        placement: 'other',
      },
    ]
  }

  const sections: SiteGuideSection[] = []
  const preamble = guide.content.slice(0, headings[0].index).trim()
  if (preamble) {
    sections.push({
      id: 'site-guide-preamble',
      title: '本站补充说明',
      content: preamble,
      placement: 'other',
    })
  }

  headings.forEach((heading, index) => {
    const title = heading[1].trim()
    const start = (heading.index || 0) + heading[0].length
    const end = headings[index + 1]?.index ?? guide.content.length
    sections.push({
      id: `site-guide-section-${index + 1}`,
      title,
      content: guide.content.slice(start, end).trim(),
      placement: getSectionPlacement(title),
    })
  })

  return sections
}

export function groupSiteGuideSections(
  sections: SiteGuideSection[]
): SiteGuideSectionsByPlacement {
  const grouped: SiteGuideSectionsByPlacement = {
    'quick-start': [],
    'api-key': [],
    'model-selection': [],
    'client-import': [],
    balance: [],
    usage: [],
    other: [],
  }

  sections.forEach((section) => grouped[section.placement].push(section))
  return grouped
}
