import path from 'node:path'
import { defineConfig } from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'

// Separate local-only entry. Never imported by the production route tree.
export default defineConfig({
  plugins: [pluginReact()],
  source: { entry: { index: './dev/capability-preview.tsx' } },
  resolve: { alias: { '@': path.resolve('./src') } },
  html: { title: '智商测试 · 本地交互验收' },
  server: { host: '127.0.0.1', port: 4197, strictPort: true },
  output: { distPath: { root: '/tmp/juxing-iq-ui-preview' } },
})
