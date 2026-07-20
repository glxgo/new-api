export interface SiteGuide {
  id: 'site-guide'
  title: string
  subtitle: string
  content: string
}

// 后台教程必须按原文完整保留。它可以没有任何 Markdown 标题，不能因为
// 页面导航需要分段而被过滤或改写。
export function getSiteGuide(content?: string | null): SiteGuide | null {
  const normalized = content?.trim() || ''
  if (!normalized) return null

  return {
    id: 'site-guide',
    title: '本站使用教程（补充）',
    subtitle: '本站端点、客户端导入与用量查看的补充步骤',
    content: normalized,
  }
}
