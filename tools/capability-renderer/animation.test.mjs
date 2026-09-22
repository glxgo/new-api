import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { renderAnimation } from './animation.mjs';

const document = (content) => `<!doctype html><html><body style="margin:0;background:white"><svg width="600" height="400">${content}</svg></body></html>`;
const hash = (value) => createHash('sha256').update(value).digest('hex');
const fixtures = {
  static: document('<circle cx="50" cy="50" r="20" fill="red"/>'),
  smil: document('<circle cx="50" cy="50" r="20" fill="red"><animate attributeName="cx" from="50" to="550" dur="5s" repeatCount="indefinite"/></circle>'),
  css: document('<style>@keyframes move{from{transform:translateX(0)}to{transform:translateX(500px)}}circle{animation:move 5s linear infinite}</style><circle cx="50" cy="50" r="20" fill="red"/>'),
  js: document('<circle id="c" cx="50" cy="50" r="20" fill="red"/><script>let x=50;setInterval(()=>{x+=10;document.getElementById("c").setAttribute("cx",x)},100)</script>'),
  external: document('<circle cx="50" cy="50" r="20"/><script>window.__capabilityBlocked=false;fetch("https://example.invalid/private").catch(()=>{})</script>'),
  localfile: document('<image href="file:///etc/passwd" width="600" height="400"/>'),
  stylesheet: document('<style>svg{background:url(https://example.invalid/image)}</style>'),
  frame: document('<foreignObject width="600" height="400"><iframe src="https://example.invalid/"></iframe></foreignObject>'),
};
process.env.CAPABILITY_RENDERER_FIXTURE_HASHES = Object.values(fixtures).map(hash).join(',');

test('untrusted local input is refused before launching Chromium', async () => {
  await assert.rejects(renderAnimation(document('<text>not allowlisted</text>')), /isolation_required/);
});
test('SVG-only output is not accepted for the HTML animation task', async () => {
  const svg = '<svg width="600" height="400"><circle cx="50" cy="50" r="20"/></svg>';
  process.env.CAPABILITY_RENDERER_FIXTURE_HASHES = `${process.env.CAPABILITY_RENDERER_FIXTURE_HASHES},${hash(svg)}`;
  await assert.rejects(renderAnimation(svg), /invalid_animation_html/);
});
for (const kind of ['static', 'smil', 'css', 'js']) {
  test(`${kind}: 48 bounded, timed pixel frames`, { timeout: 30000 }, async () => {
    const result = await renderAnimation(fixtures[kind]);
    assert.equal(result.frames.length, 48);
    assert.equal(result.step_ms, 100);
    assert.match(result.browser, /^\d+\./);
    const hashes = new Set(result.frames.map(hash));
    assert.equal(hashes.size > 1, kind !== 'static');
    for (const frame of result.frames) {
      const png = Buffer.from(frame, 'base64');
      assert.equal(png.readUInt32BE(16), 600);
      assert.equal(png.readUInt32BE(20), 400);
    }
  });
}
for (const kind of ['external', 'localfile', 'stylesheet', 'frame']) {
  test(`${kind}: external dependencies are rejected outside the page`, { timeout: 30000 }, async () => {
    await assert.rejects(renderAnimation(fixtures[kind]), /external_resource_blocked/);
  });
}
