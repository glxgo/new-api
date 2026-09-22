import { useState } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { i18nReady } from '@/i18n/config'
import '@/styles/index.css'
import i18next from 'i18next'
import { Toaster } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api'
import { FontProvider } from '@/context/font-provider'
import { ThemeProvider, useTheme } from '@/context/theme-provider'
import { Alert, AlertTitle, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { PelicanArchivePage } from '@/features/pelican-archive'
import { PelicanArchiveAdmin } from '@/features/pelican-archive/admin'

const client = new QueryClient({
  defaultOptions: { queries: { retry: false } },
})
function Preview(props: {
  mode: string
  capturedAt: string
  userGroup: string
}) {
  const { theme, setTheme } = useTheme()
  const [tab, setTab] = useState('admin')
  const context =
    props.mode === 'production_snapshot'
      ? `渠道、分组和倍率来自生产只读快照（${new Date(props.capturedAt).toLocaleString()}），采用已确认的关联；非实时拓扑。用户页面按本地测试账号所属分组 ${props.userGroup} 的权限展示，管理后台可预览全部分组。`
      : '分组与渠道关联为本地演示数据。'
  return (
    <main className='mx-auto flex max-w-7xl flex-col gap-4 p-4'>
      <Alert>
        <AlertTitle>鹈鹕存档 · 本地验收</AlertTitle>
        <AlertDescription>
          作品和判定来自外部服务器真实存档。{context}{' '}
          操作仅影响内存数据库，未上线，不发起模型调用。
        </AlertDescription>
      </Alert>
      <Button
        variant='outline'
        onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
      >
        切换明暗主题
      </Button>
      <Tabs value={tab} onValueChange={(v) => setTab(String(v))}>
        <TabsList>
          <TabsTrigger value='admin'>管理后台</TabsTrigger>
          <TabsTrigger value='user'>用户页面</TabsTrigger>
        </TabsList>
        <TabsContent value='admin'>
          <PelicanArchiveAdmin />
        </TabsContent>
        <TabsContent value='user'>
          <PelicanArchivePage />
        </TabsContent>
      </Tabs>
    </main>
  )
}
async function start() {
  await i18nReady
  await i18next.changeLanguage('zh')
  const { data } = await api.get('/api/local-acceptance-session')
  window.localStorage.setItem('uid', String(data.id))
  useAuthStore.getState().auth.setUser(data)
  createRoot(
    document.getElementById('root') ||
      document.body.appendChild(document.createElement('div'))
  ).render(
    <QueryClientProvider client={client}>
      <ThemeProvider>
        <FontProvider>
          <Preview
            mode={data.preview_mode}
            capturedAt={data.topology_captured_at}
            userGroup={data.local_user_group}
          />
          <Toaster />
        </FontProvider>
      </ThemeProvider>
    </QueryClientProvider>
  )
}
void start()
