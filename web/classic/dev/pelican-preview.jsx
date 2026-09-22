import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Banner, Button, Tabs, TabPane } from '@douyinfe/semi-ui';
import '@douyinfe/semi-ui/dist/css/semi.css';
import '../src/index.css';
import '../src/i18n/i18n';
import { API } from '../src/helpers';
import {
  ThemeProvider,
  useActualTheme,
  useSetTheme,
} from '../src/context/Theme';
import PelicanArchive from '../src/pages/PelicanArchive';
import PelicanSettings from '../src/pages/PelicanArchive/settings';
function Preview({ session }) {
  const theme = useActualTheme(),
    setTheme = useSetTheme();
  const [tab, setTab] = useState('user');
  const context =
    session.preview_mode === 'production_snapshot'
      ? `渠道、分组和倍率来自生产只读快照（${new Date(session.topology_captured_at).toLocaleString()}），采用已确认关联；非实时拓扑。用户页面按本地测试账号所属分组 ${session.local_user_group} 的权限展示，管理后台可预览全部分组。`
      : '演示分组关联。';
  return (
    <main style={{ maxWidth: 1280, margin: '0 auto', padding: 16 }}>
      <Banner
        description={`本地验收：真实外部存档。${context} 操作仅影响内存数据库，未上线，不调用模型。`}
      />
      <Button onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}>
        切换明暗主题
      </Button>
      <Tabs activeKey={tab} onChange={setTab}>
        <TabPane tab='用户页面' itemKey='user'>
          <PelicanArchive />
        </TabPane>
        <TabPane tab='管理后台' itemKey='admin'>
          <PelicanSettings />
        </TabPane>
      </Tabs>
    </main>
  );
}
async function start() {
  const r = await API.get('/api/local-acceptance-session');
  localStorage.setItem('user', JSON.stringify(r.data));
  API.defaults.headers['New-API-User'] = String(r.data.id);
  createRoot(document.getElementById('root')).render(
    <ThemeProvider>
      <Preview session={r.data} />
    </ThemeProvider>,
  );
}
void start();
