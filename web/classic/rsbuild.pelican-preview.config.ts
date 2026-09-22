import { defineConfig, mergeRsbuildConfig } from '@rsbuild/core'
import base from './rsbuild.config'
export default defineConfig((env) => mergeRsbuildConfig(base(env), {
 source: { entry: { index: './dev/pelican-preview.jsx' } },
 server: { host: '127.0.0.1', port: 4200, strictPort: true, proxy: { '/api': { target: 'http://127.0.0.1:4198', changeOrigin: true } } },
 output: { distPath: { root: 'dist-pelican-preview' } },
}))
