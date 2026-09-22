import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdtemp, rm } from 'node:fs/promises';
import http from 'node:http';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const good = '<!doctype html><html><body><svg width="600" height="400"><circle cx="50" cy="50" r="20"/></svg></body></html>';
const hung = '<!doctype html><html><body><svg width="600" height="400"></svg><script>while(true){}</script></body></html>';
test('hung scripts and cancelled clients release the isolated renderer', { timeout: 55000 }, async () => {
  const dir = await mkdtemp('/tmp/iq-node-');
  const socketPath = path.join(dir, 'renderer.sock');
  const server = spawn(process.execPath, ['server.mjs'], {
    cwd: path.dirname(fileURLToPath(import.meta.url)),
    env: { PATH: process.env.PATH, RENDERER_SOCKET: socketPath,
      PLAYWRIGHT_BROWSERS_PATH: process.env.PLAYWRIGHT_BROWSERS_PATH,
      CAPABILITY_RENDERER_FIXTURE_HASHES: [good, hung].map(v => createHash('sha256').update(v).digest('hex')).join(',') },
    stdio: 'ignore',
  });
  const request = (body, cancel = false) => new Promise((resolve, reject) => {
    const req = http.request({ socketPath, path: '/animation', method: 'POST' }, (res) => {
      res.resume(); res.once('end', () => resolve(res.statusCode));
    });
    req.on('error', reject); req.end(body);
    if (cancel) { const timer = setTimeout(() => req.destroy(new Error('client_cancel')), 1500); timer.unref(); }
  });
  try {
    for (let i = 0; ; i++) {
      try { assert.equal(await request('invalid'), 422); break; }
      catch (e) { if (i >= 50) throw e; await new Promise(r => setTimeout(r, 50)); }
    }
    await assert.rejects(request(hung, true), /client_cancel/);
    let status;
    for (let i = 0; i < 50; i++) {
      status = await request(good);
      if (status !== 429) break;
      await new Promise(r => setTimeout(r, 50));
    }
    assert.equal(status, 200);
    const started = Date.now();
    assert.equal(await request(hung), 422);
    assert.ok(Date.now() - started < 33000, 'parent deadline must stop infinite JS');
    assert.equal(await request(good), 200);
  } finally {
    server.kill('SIGTERM');
    await new Promise(resolve => server.once('exit', resolve));
    await rm(dir, { recursive: true });
  }
});
