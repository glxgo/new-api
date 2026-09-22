import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { assertIsolation } from './isolation.mjs';

const html = '<html><svg/></html>';
const digest = createHash('sha256').update(html).digest('hex');
const linux = { platform: 'linux', uid: 65532, externalNetwork: false, isolated: 'true', fixtures: '' };

test('isolated mode requires Linux, non-root and no external interface', () => {
  assert.doesNotThrow(() => assertIsolation(html, linux));
  for (const change of [{ platform: 'darwin' }, { uid: 0 }, { uid: undefined }, { externalNetwork: true }, { isolated: '' }]) {
    assert.throws(() => assertIsolation(html, { ...linux, ...change }), /isolation_required/);
  }
});

test('fixture allowlist cannot bypass a deployment isolation check', () => {
  assert.throws(() => assertIsolation(html, { ...linux, fixtures: digest }), /fixture_override_forbidden/);
  for (const change of [{ uid: 0 }, { externalNetwork: true }, { isolated: '' }, { platform: 'darwin' }]) {
    assert.throws(() => assertIsolation(html, { ...linux, fixtures: digest, ...change }));
  }
  const mac = { ...linux, platform: 'darwin', isolated: '', fixtures: digest };
  assert.doesNotThrow(() => assertIsolation(html, mac));
  assert.throws(() => assertIsolation('not allowlisted', mac), /isolation_required/);
});
