import { createHash } from 'node:crypto';
import { networkInterfaces } from 'node:os';

export function assertIsolation(html, runtime = {
  platform: process.platform,
  uid: process.getuid?.(),
  externalNetwork: Object.values(networkInterfaces()).flat().some(v => v && !v.internal),
  isolated: process.env.CAPABILITY_RENDERER_ISOLATED,
  fixtures: process.env.CAPABILITY_RENDERER_FIXTURE_HASHES,
}) {
  // The fixture escape hatch belongs only to the macOS developer workstation.
  // It must never override isolation on Linux or an isolated-mode deployment.
  if (runtime.platform === 'darwin' && runtime.isolated !== 'true') {
    const digest = createHash('sha256').update(html).digest('hex');
    if ((runtime.fixtures || '').split(',').includes(digest)) return;
  }
  if (runtime.platform !== 'linux' || !Number.isInteger(runtime.uid) || runtime.uid <= 0 || runtime.externalNetwork || runtime.isolated !== 'true') {
    throw new Error('animation_isolation_required');
  }
  if (runtime.fixtures) throw new Error('animation_fixture_override_forbidden');
}
