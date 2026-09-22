import { renderAnimation } from './animation.mjs';
process.once('message', async (html) => {
  try { process.send({ ok: true, result: await renderAnimation(html, pid => process.send({ browser_pid: pid })) }, () => process.exit(0)); }
  catch { process.send({ ok: false }, () => process.exit(1)); }
});
