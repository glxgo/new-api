import test from 'node:test';
import assert from 'node:assert/strict';
import { Worker } from 'node:worker_threads';

function render(svg) {
  return new Promise((resolve, reject) => {
    const worker = new Worker(new URL('./render.mjs', import.meta.url), { workerData: svg });
    const timer = setTimeout(() => { worker.terminate(); reject(new Error('timeout')); }, 8000);
    worker.once('message', data => { clearTimeout(timer); worker.terminate(); resolve(Buffer.from(data)); });
    worker.once('error', error => { clearTimeout(timer); reject(error); });
  });
}
test('static SVG renders as a bounded PNG', async () => {
  const png = await render('<svg xmlns="http://www.w3.org/2000/svg" width="600" height="400" viewBox="0 0 600 400"><rect width="600" height="400" fill="white"/><circle cx="100" cy="100" r="30" fill="red"/></svg>');
  assert.equal(png.subarray(1,4).toString(), 'PNG'); assert.equal(png.readUInt32BE(16),600); assert.equal(png.readUInt32BE(20),400);
});
test('external files, URLs and executable content are rejected', async () => {
  for (const body of ['<image href="file:///etc/passwd"/>','<script>alert(1)</script>','<rect fill="url(https://example.com)"/>']) {
    await assert.rejects(render(`<svg width="600" height="400">${body}</svg>`));
  }
});
