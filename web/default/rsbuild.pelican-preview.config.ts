import path from 'node:path'
import { defineConfig } from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'
export default defineConfig({
 plugins: [pluginReact()],
 source: { entry: { index: './dev/pelican-preview.tsx' } },
 resolve: { alias: { '@': path.resolve(__dirname, './src') } },
 server: { host: '127.0.0.1', port: 4199, strictPort: true, proxy: { '/api': { target: 'http://127.0.0.1:4198', changeOrigin: true } } },
 output: { distPath: { root: 'dist-pelican-preview' } },
})
