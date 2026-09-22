// Local-only acceptance of the real background importer. The caller first
// atomically replaces the preview's file with a newly fetched real snapshot.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
const snapshot = JSON.parse(await readFile(process.argv[2], 'utf8'));
const origin = 'http://127.0.0.1:4198';
const login = await fetch(origin + '/api/local-acceptance-session');
const session = await login.json();
assert.equal(session.preview_mode, 'production_snapshot');
const headers = { cookie: login.headers.get('set-cookie').split(';')[0], 'New-API-User': String(session.id), 'Content-Type': 'application/json' };
const base = origin + '/api/pelican-archive';
async function current() {
  const response = await fetch(base + '/admin/control', { headers });
  const body = await response.json();
  assert(body.success);
  return body.data;
}
async function interval(minutes) {
  const state = await current();
  const response = await fetch(base + '/admin/control', { method: 'PUT', headers, body: JSON.stringify({ action: 'interval', revision: state.control.revision, interval_minutes: minutes }) });
  assert((await response.json()).success);
}
const before = await current();
assert(before.control.sync_enabled);
assert(new Date(snapshot.captured_at) > new Date(before.control.last_captured_at), 'use a new snapshot before the worker imports it');
try {
  await interval(1);
  const deadline = Date.now() + 95000;
  let after;
  while (Date.now() < deadline) {
    after = await current();
    if (after.control.last_captured_at === snapshot.captured_at) break;
    await new Promise(resolve => setTimeout(resolve, 2000));
  }
  assert.equal(after.control.last_captured_at, snapshot.captured_at, 'background worker imports without a manual sync request');
  assert.equal(after.source_config.interval_minutes, snapshot.config.interval_minutes);
  assert.deepEqual(after.targets.map(t => [t.id, t.channel_id, t.display_model, t.hidden]), before.targets.map(t => [t.id, t.channel_id, t.display_model, t.hidden]));
  console.log(JSON.stringify({ before: before.control.last_captured_at, after: after.control.last_captured_at, last_imported_at: after.control.last_imported_at, automatic: true, mappings_preserved: true }, null, 2));
} finally {
  await interval(before.control.interval_minutes);
}
