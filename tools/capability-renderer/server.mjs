import http from 'node:http';
import { Worker } from 'node:worker_threads';
import { chmodSync } from 'node:fs';
import { fork } from 'node:child_process';

const socket = process.env.RENDERER_SOCKET || '/run/capability/renderer.sock';
let busy = false;
let activeKill = () => {};
process.on('SIGTERM', () => { activeKill(); process.exit(0); });
process.on('SIGINT', () => { activeKill(); process.exit(0); });
const server = http.createServer(async (req, res) => {
  if (req.method !== 'POST' || !['/render', '/animation'].includes(req.url)) { res.writeHead(404).end(); return; }
  if (busy) { res.writeHead(429).end(); return; }
  busy = true;
  let worker;
  let timer;
  let child;
  let browserPID;
  const animation = req.url === '/animation';
  // Chromium owns a separate process group; kill it too on a hung script.
  const killChild = () => {
    for (const pid of [browserPID, child?.pid]) {
      if (Number.isSafeInteger(pid) && pid > 1) { try { process.kill(-pid, 'SIGKILL'); } catch {} }
    }
  };
  activeKill = killChild;
  try {
    const chunks = []; let bytes = 0;
    for await (const chunk of req) { bytes += chunk.length; if (bytes > (animation ? 409600 : 204800)) throw new Error('Too large'); chunks.push(chunk); }
    if (animation) {
      const result = await new Promise((resolve, reject) => {
        child = fork(new URL('./animation-child.mjs', import.meta.url), [], {
          detached: true, serialization: 'json', stdio: ['ignore', 'ignore', 'ignore', 'ipc'],
          env: { PATH: process.env.PATH, PLAYWRIGHT_BROWSERS_PATH: process.env.PLAYWRIGHT_BROWSERS_PATH, CAPABILITY_RENDERER_FIXTURE_HASHES: process.env.CAPABILITY_RENDERER_FIXTURE_HASHES, CAPABILITY_RENDERER_ISOLATED: process.env.CAPABILITY_RENDERER_ISOLATED },
          execArgv: ['--max-old-space-size=192'],
        });
        child.on('message', (v) => {
          if (Number.isSafeInteger(v.browser_pid) && v.browser_pid > 1) { browserPID = v.browser_pid; return; }
          v.ok ? resolve(v.result) : reject(new Error('Animation failed'));
        });
        child.once('error', reject);
        child.once('exit', code => { if (code !== 0) reject(new Error('Animation exited')); });
        res.once('close', () => { if (!res.writableEnded) { killChild(); reject(new Error('Cancelled')); } });
        timer = setTimeout(() => { killChild(); reject(new Error('Animation timeout')); }, 30000);
        child.send(Buffer.concat(chunks).toString('utf8'));
      });
      res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' }).end(JSON.stringify(result));
      return;
    }
    const png = await new Promise((resolve, reject) => {
      worker = new Worker(new URL('./render.mjs', import.meta.url), {
        workerData: Buffer.concat(chunks).toString('utf8'),
        resourceLimits: { maxOldGenerationSizeMb: 64, stackSizeMb: 2 },
      });
      worker.once('message', resolve);
      worker.once('error', reject);
      worker.once('exit', code => { if (code !== 0) reject(new Error('Render failed')); });
      timer = setTimeout(() => reject(new Error('Render timeout')), 8000);
    });
    res.writeHead(200, { 'Content-Type': 'image/png', 'Cache-Control': 'no-store' }).end(Buffer.from(png));
  } catch { if (!res.headersSent) res.writeHead(422).end(); }
  finally { clearTimeout(timer); killChild(); await worker?.terminate(); activeKill = () => {}; busy = false; }
});
server.requestTimeout = 10000;
server.headersTimeout = 10000;
// Fail on an existing socket; never unlink an arbitrary configured path.
server.listen(socket, () => chmodSync(socket, 0o660));
