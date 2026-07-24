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
import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const sourceRoot = path.join(root, 'src')
const forbiddenImports = [
  {
    pattern: /from\s+['"]@hugeicons\/core-free-icons['"]/,
    message:
      'Import Hugeicons from its tree-shakeable dist/esm/index subpath, not the production-minified package root.',
  },
  {
    pattern: /from\s+['"]@lobehub\/icons['"]/,
    message:
      'Import Lobe icons from explicit provider subpaths instead of the package barrel.',
  },
]

function walk(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const target = path.join(directory, entry.name)
    if (entry.isDirectory()) return walk(target)
    return /\.(?:ts|tsx)$/.test(entry.name) ? [target] : []
  })
}

const failures = []
for (const file of walk(sourceRoot)) {
  const source = fs.readFileSync(file, 'utf8')
  const relativeFile = path.relative(root, file)
  for (const rule of forbiddenImports) {
    if (rule.pattern.test(source)) {
      failures.push(`${relativeFile}: ${rule.message}`)
    }
  }

  if (
    relativeFile.startsWith(
      `src${path.sep}routes${path.sep}_authenticated${path.sep}system-settings`
    ) &&
    /section-registry/.test(source)
  ) {
    failures.push(
      `${relativeFile}: Route guards must import lightweight section-meta modules.`
    )
  }

  if (
    relativeFile ===
      `src${path.sep}components${path.sep}layout${path.sep}config${path.sep}system-settings.config.ts` &&
    /section-registry/.test(source)
  ) {
    failures.push(
      `${relativeFile}: Shared navigation must use lightweight section-meta modules.`
    )
  }

  if (
    relativeFile === `src${path.sep}i18n${path.sep}config.ts` &&
    /import\s+\w+\s+from\s+['"].\/locales\//.test(source)
  ) {
    failures.push(
      `${relativeFile}: Locale JSON must stay behind dynamic imports.`
    )
  }

  if (
    relativeFile ===
      `src${path.sep}features${path.sep}pricing${path.sep}index.tsx` &&
    /import\s*\{[\s\S]*ModelDetailsDrawer[\s\S]*\}\s*from\s*['"].\/components['"]/.test(
      source
    )
  ) {
    failures.push(
      `${relativeFile}: The model details drawer must stay behind React.lazy.`
    )
  }

  if (
    relativeFile ===
      `src${path.sep}features${path.sep}pricing${path.sep}components${path.sep}model-details.tsx` &&
    /from\s+['"].\/model-details-(?:api|performance)['"]/.test(source)
  ) {
    failures.push(
      `${relativeFile}: Heavy model detail tabs must stay behind dynamic imports.`
    )
  }

  if (
    relativeFile ===
      `src${path.sep}features${path.sep}dashboard${path.sep}lib${path.sep}charts.ts` &&
    /from\s+['"]@visactor\//.test(source)
  ) {
    failures.push(
      `${relativeFile}: Chart data processing must not import the VChart runtime.`
    )
  }

  if (
    relativeFile ===
      `src${path.sep}features${path.sep}profit${path.sep}index.tsx` &&
    /from\s+['"].\/components\/profit-chart['"]/.test(source)
  ) {
    failures.push(
      `${relativeFile}: The profit chart must stay behind React.lazy.`
    )
  }

  if (
    relativeFile ===
      `src${path.sep}features${path.sep}rankings${path.sep}index.tsx` &&
    /from\s+['"].\/components\/(?:models|market-share)-section['"]/.test(source)
  ) {
    failures.push(
      `${relativeFile}: Ranking chart sections must stay behind React.lazy.`
    )
  }

  if (
    relativeFile !==
      `src${path.sep}lib${path.sep}status-resource.ts` &&
    (/import\s*\{[^}]*\bgetStatus\b[^}]*\}\s*from\s*['"]@\/lib\/api['"]/.test(
      source
    ) ||
      /fetch\s*\(\s*['"]\/api\/status['"]/.test(source))
  ) {
    failures.push(
      `${relativeFile}: System status requests must use the shared status resource.`
    )
  }

  if (
    relativeFile ===
      `src${path.sep}routes${path.sep}_authenticated${path.sep}channels${path.sep}index.tsx` &&
    !/prefetchQuery\s*\(\s*channelListQueryOptions/.test(source)
  ) {
    failures.push(
      `${relativeFile}: Channels data must warm during route intent preloading.`
    )
  }

  if (
    relativeFile ===
      `src${path.sep}routes${path.sep}_authenticated${path.sep}usage-logs${path.sep}$section.tsx` &&
    !/prefetchQuery\s*\(\s*usageLogsQueryOptions/.test(source)
  ) {
    failures.push(
      `${relativeFile}: Usage Logs data must warm during route intent preloading.`
    )
  }
}

if (failures.length > 0) {
  console.error(failures.join('\n'))
  process.exitCode = 1
} else {
  console.log('Performance import guard passed.')
}
