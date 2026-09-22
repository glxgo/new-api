import { parentPort, workerData } from 'node:worker_threads';
import { Resvg } from '@resvg/resvg-js';

// No browser, script engine, URL fetcher, channel credentials or mounted
// application data. Each render has a separate worker and deadline.
const svg = workerData;
if (typeof svg !== 'string' || Buffer.byteLength(svg) > 204800 ||
    /<!DOCTYPE|<!ENTITY|<\s*(script|image|foreignObject|use|style|a)\b|(?:href|src)\s*=|url\s*\(/i.test(svg)) {
  throw new Error('Unsupported SVG');
}
const renderer = new Resvg(svg, {
  font: { loadSystemFonts: false, defaultFontFamily: 'DejaVu Sans', fontDirs: ['/fonts'] },
  fitTo: { mode: 'width', value: 600 },
  background: '#ffffff',
});
if (renderer.width !== 600 || renderer.height !== 400) throw new Error('Invalid canvas');
parentPort.postMessage(renderer.render().asPng());
