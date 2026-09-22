// Real local HTTP acceptance. Writes only to the isolated preview harness,
// restoring its original presentation, mappings and switches in finally.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';

const [topologyPath, archivePath, mappingsPath] = process.argv.slice(2);
assert(topologyPath && archivePath && mappingsPath, 'Supply topology, archive and confirmed mappings JSON paths');
const read = async (path) => JSON.parse(await readFile(path, 'utf8'));
const [topology, archive, mappings] = await Promise.all([read(topologyPath), read(archivePath), read(mappingsPath)]);
const origin = 'http://127.0.0.1:4198'; // Deliberately not configurable to a server.
const login = await fetch(origin + '/api/local-acceptance-session');
const session = await login.json();
assert.equal(session.preview_mode, 'production_snapshot');
assert.equal(new Date(session.topology_captured_at).getTime(), Math.floor(new Date(topology.captured_at).getTime() / 1000) * 1000);
const headers = { cookie: login.headers.get('set-cookie').split(';')[0], 'New-API-User': String(session.id), 'Content-Type': 'application/json' };
const base = origin + '/api/pelican-archive';
async function get(path) {
  const r = await fetch(base + path, { headers });
  assert.equal(r.status, 200, path);
  const j = await r.json();
  assert.equal(j.success, true, path);
  return j;
}
const admin = () => get('/admin/control').then(r => r.data);
async function update(action, payload = {}) {
  const current = await admin();
  const r = await fetch(base + '/admin/control', { method: 'PUT', headers, body: JSON.stringify({ ...payload, action, revision: current.control.revision }) });
  const j = await r.json();
  assert.equal(j.success, true, `local update ${action}`);
}
const original = await admin();
assert.equal(original.targets.length, mappings.bindings.length);
assert.deepEqual(original.channels, [...topology.channels].sort((a, b) => a.id - b.id));
const ratios = JSON.parse(topology.options.GroupRatio);
let disabled = 0, relationships = 0, verifiedArtworks = 0;
for (const b of mappings.bindings) {
  const target = original.targets.find(t => t.provider_id === b.provider_id && t.model_name === b.model);
  assert(target, 'confirmed target exists');
  assert.equal(target.channel_id, b.channel_id);
  const channel = topology.channels.find(c => c.id === b.channel_id);
  const report = original.mapping_reports.find(r => r.target_id === target.id);
  assert.equal(report.reason, '');
  assert.equal(report.channel_disabled, channel.status !== 1);
  if (report.channel_disabled) disabled++;
  assert.deepEqual(report.groups.map(g => g.routing_key).sort(), [...new Set(channel.group.split(',').filter(Boolean))].sort());
  for (const g of report.groups) {
    assert.equal(g.ratio, ratios[g.routing_key]);
    const ability = topology.abilities.find(a => a.channel_id === b.channel_id && a.group === g.routing_key && a.model === b.model);
    assert(ability && (ability.enabled || channel.status !== 1));
    assert(g.eligible);
    relationships++;
  }
}
const publicGroups = (await get('/groups')).data;
const preview = (await get('/admin/preview')).data;
const tested = preview.filter(g => g.models.length);
assert(tested.length > 1);
const records = new Map(), used = new Map();
for (const g of tested) {
  assert.deepEqual(g.models, ['gpt-6-astra']);
  assert.equal(g.display_name, g.routing_key);
  assert.equal(g.ratio, ratios[g.routing_key]);
  const result = (await get(`/admin/groups/${g.group_uid}/results?model=gpt-6-astra`)).data;
  assert(result.gallery.length > 0 && result.gallery.length <= 3);
  assert.equal(new Set(result.gallery.map(r => r.id)).size, result.gallery.length);
  for (const r of result.gallery) {
    used.set(r.id, (used.get(r.id) || 0) + 1);
    if (records.has(r.id)) continue;
    const detail = (await get(`/admin/records/${r.id}`)).data;
    const source = archive.runs.find(s => s.id === detail.source_record.id);
    assert(source, 'record traceable to actual source snapshot');
    assert.deepEqual(detail.source_record, source);
    const latest = Math.max(...archive.runs.filter(s => s.provider_id === source.provider_id && s.model_name === source.model_name).map(s => s.id));
    assert.equal(source.id, latest, 'never substitute old good result for latest result');
    if (r.has_artwork) {
      const art = await fetch(base + `/admin/records/${r.id}/artwork`, { headers });
      assert.equal(art.status, 200);
      assert.equal(await art.text(), source.svg, 'original SVG bytes unchanged');
      verifiedArtworks++;
    }
    records.set(r.id, detail);
  }
}
assert([...used.values()].some(n => n > 1), 'cross-group shared record IDs');
try {
  const target = original.targets.find(t => t.channel_id === 83);
  const mapping = { target_id: target.id, channel_id: target.channel_id, display_model: target.display_model, hidden: true };
  await update('mapping', mapping);
  assert.equal((await admin()).mapping_reports.find(r => r.target_id === target.id).reason, 'target_hidden');
  assert.deepEqual((await admin()).channels, original.channels, 'hiding never changes business channel');
  await update('mapping', { ...mapping, hidden: false });
  const g = publicGroups.find(g => g.models.length);
  const presentation = structuredClone(original.presentation);
  presentation.groups[g.group_uid] = { name: '本地验收临时别名', description_mode: 'custom', description: '本地验收临时描述' };
  presentation.copy.page_title = '本地验收临时标题';
  presentation.order = [...preview.map(g => g.group_uid)].reverse();
  await update('presentation', { presentation });
  const edited = await get('/groups');
  const changed = edited.data.find(x => x.group_uid === g.group_uid);
  assert.equal(changed.display_name, '本地验收临时别名');
  assert.equal(changed.description, '本地验收临时描述');
  assert.equal(changed.routing_key, g.routing_key);
  assert.equal(changed.ratio, g.ratio);
  assert.equal(edited.presentation.copy.page_title, presentation.copy.page_title);
  const expectedOrder = presentation.order.filter(id => publicGroups.some(g => g.group_uid === id));
  assert.deepEqual(edited.data.map(g => g.group_uid), expectedOrder);
  await update('interval', { interval_minutes: 7 });
  assert.equal((await admin()).control.interval_minutes, 7);
  await update('hide');
  assert.equal((await get('/groups')).visible, false);
  assert.equal((await admin()).control.sync_enabled, true);
  assert.equal((await get('/admin/preview')).visible, true);
  await update('show');
  await update('pause');
  assert.equal((await get('/groups')).visible, true);
  assert.equal((await admin()).control.sync_enabled, false);
  await update('hide_and_pause');
  assert.equal((await get('/groups')).visible, false);
  assert.equal((await admin()).control.sync_enabled, false);
} finally {
  await update('presentation', { presentation: original.presentation });
  for (const t of original.targets) {
    const current = (await admin()).targets.find(x => x.id === t.id);
    if (current.hidden !== t.hidden) await update('mapping', { target_id: t.id, channel_id: t.channel_id, display_model: t.display_model, hidden: t.hidden });
  }
  await update('interval', { interval_minutes: original.control.interval_minutes });
  await update(original.control.visible ? 'show' : 'hide');
  await update(original.control.sync_enabled ? 'resume' : 'pause');
}
const final = await admin();
assert.deepEqual(final.channels, original.channels);
assert.deepEqual(final.presentation, original.presentation);
console.log(JSON.stringify({
  topology_captured_at: topology.captured_at, archive_captured_at: archive.captured_at,
  channels: original.channels.length, abilities: topology.abilities.length,
  confirmed_targets: original.targets.length, disabled_targets_eligible: disabled,
  eligible_memberships: relationships, admin_test_groups: tested.length,
  user_test_groups: publicGroups.filter(g => g.models.length).length,
  unique_gallery_records: records.size, original_svg_verified: verifiedArtworks,
  shared_records: [...used.values()].filter(n => n > 1).length,
  archive_sha256: createHash('sha256').update(await readFile(archivePath)).digest('hex'),
  settings_restored: true, production_writes: false,
}, null, 2));
