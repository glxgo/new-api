import { chromium } from 'playwright';
import { assertIsolation } from './isolation.mjs';

export const animationVersion = 'chromium-animation-1';
export const frameCount = 48;
export const frameStepMs = 100;
const origin = 'http://capability-render.invalid/';
const policy = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data:; font-src 'none'; connect-src 'none'; media-src 'none'; object-src 'none'; frame-src 'none'; worker-src 'none'; base-uri 'none'; form-action 'none'; sandbox allow-scripts";

// Local builds can render only explicitly allowlisted, developer-owned fixtures.
// Production must use a dedicated, non-root, network:none container. Chromium's
// sandbox remains enabled; no user browser, profile, cookies or business env.
export async function renderAnimation(html, onBrowser = () => {}) {
  if (typeof html !== 'string' || Buffer.byteLength(html) > 409600 || !/<(?:!doctype\s+html|html|body)\b/i.test(html) || !/<svg\b/i.test(html)) throw new Error('invalid_animation_html');
  assertIsolation(html);
  const browser = await chromium.launch({
    headless: true,
    chromiumSandbox: true,
    timeout: 10000,
    env: { PATH: process.env.PATH || '/usr/bin:/bin', LANG: 'C.UTF-8', TZ: 'UTC' },
  });
  try {
    const system = await browser.newBrowserCDPSession();
    const { processInfo } = await system.send('SystemInfo.getProcessInfo');
    const pid = processInfo.find((entry) => entry.type === 'browser')?.id;
    if (!Number.isSafeInteger(pid) || pid <= 1) throw new Error('browser_identity_unavailable');
    onBrowser(pid);
    const context = await browser.newContext({ viewport: { width: 600, height: 400 }, deviceScaleFactor: 1, locale: 'en-US', timezoneId: 'UTC', colorScheme: 'light', reducedMotion: 'no-preference', serviceWorkers: 'block', acceptDownloads: false, permissions: [] });
    let supplied = false;
    let blocked = false;
    await context.route('**/*', async (route) => {
      if (!supplied && route.request().isNavigationRequest() && route.request().url() === origin) {
        supplied = true;
        await route.fulfill({ status: 200, contentType: 'text/html; charset=utf-8', headers: { 'Content-Security-Policy': policy, 'Cache-Control': 'no-store' }, body: html });
      } else { blocked = true; await route.abort(); }
    });
    const page = await context.newPage();
    // Chromium also blocks file:// before CSP in some cases; browser console
    // security diagnostics cannot be suppressed by overriding window.console.
    page.on('console', message => {
      if (/Not allowed to load local resource|Refused to (?:load|connect|frame)/i.test(message.text())) blocked = true;
    });
    page.setDefaultTimeout(5000);
    page.on('dialog', (dialog) => dialog.dismiss());
    context.on('page', (popup) => { if (popup !== page) { blocked = true; void popup.close(); } });
    await page.clock.install({ time: new Date('2026-01-01T00:00:00Z') });
    // Observe browser-generated violations outside the untrusted page. A page
    // variable or event handler could be overwritten by the submitted script.
    const audit = await context.newCDPSession(page);
    await audit.send('Audits.enable');
    audit.on('Audits.issueAdded', ({ issue }) => {
      if (issue.code === 'ContentSecurityPolicyIssue') blocked = true;
    });
    await page.goto(origin, { waitUntil: 'load', timeout: 8000 });
    const hasSVG = await page.locator('svg').count();
    if (!hasSVG) throw new Error('svg_required');
    const frames = [];
    let bytes = 0;
    for (let i = 0; i < frameCount; i++) {
      if (i) await page.clock.runFor(frameStepMs);
      // CSS and SMIL are frozen to the same virtual timeline as JS timers.
      await page.evaluate((ms) => {
        for (const animation of document.getAnimations()) { animation.pause(); animation.currentTime = ms; }
        for (const svg of document.querySelectorAll('svg')) { svg.pauseAnimations(); svg.setCurrentTime(ms / 1000); }
      }, i * frameStepMs);
      const png = await page.screenshot({ type: 'png', animations: 'allow', caret: 'hide', timeout: 5000 });
      bytes += png.length;
      if (bytes > 12 * 1024 * 1024) throw new Error('animation_output_too_large');
      frames.push(png.toString('base64'));
    }
    if (blocked) throw new Error('external_resource_blocked');
    return { version: animationVersion, browser: browser.version(), step_ms: frameStepMs, frames };
  } finally {
    await browser.close();
  }
}
