package middleware

// mainlandBlockedPage 451 提示页（PRD v0.4 附录 A）。
// 页面完全由服务端直出，不依赖站内 JS、CSS、字体或图片资源，避免受限请求再次触发前端加载。
const mainlandBlockedPage = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="theme-color" content="#0b0c10">
<meta name="robots" content="noindex, nofollow">
<title>本站点不向中国大陆地区提供服务</title>
<style>
  :root { color-scheme: dark; }
  * { box-sizing: border-box; }
  html, body { min-height: 100%; }
  body {
    margin: 0;
    min-height: 100vh;
    padding: 24px 16px;
    display: grid;
    place-items: center;
    background: #0b0c10;
    color: #ececf0;
    font-family: -apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
  }
  .card {
    position: relative;
    width: min(800px, 100%);
    min-height: min(780px, calc(100vh - 48px));
    overflow: hidden;
    padding: 58px 58px 48px;
    border: 1px solid #2c3342;
    border-radius: 30px;
    background: linear-gradient(145deg, #151821 0%, #10131b 58%, #0e1016 100%);
    box-shadow: 0 24px 80px rgba(0, 0, 0, .35), inset 0 1px 0 rgba(255, 255, 255, .035);
  }
  .card::before {
    content: "";
    position: absolute;
    inset: 0 0 auto;
    height: 2px;
    background: linear-gradient(90deg, transparent 0%, #c79a43 18%, #d7b263 50%, #c79a43 82%, transparent 100%);
    opacity: .9;
  }
  .watermark {
    position: absolute;
    top: 26px;
    right: 4.2%;
    color: rgba(255,255,255,.035);
    font-size: clamp(112px, 18vw, 210px);
    font-weight: 800;
    line-height: 1;
    letter-spacing: -.08em;
    user-select: none;
    pointer-events: none;
  }
  .tag {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 12px;
    padding: 9px 20px;
    border: 2px solid rgba(199, 154, 67, .58);
    border-radius: 999px;
    color: #d6ad60;
    font-size: clamp(14px, 1.35vw, 18px);
    letter-spacing: .08em;
  }
  .tag svg { width: 21px; height: 21px; flex: none; }
  h1 {
    position: relative;
    margin: 42px 0 24px;
    font-size: clamp(28px, 2.65vw, 38px);
    line-height: 1.2;
    font-weight: 750;
    letter-spacing: .01em;
  }
  h1::after {
    content: "";
    display: block;
    width: 88px;
    height: 5px;
    margin-top: 24px;
    border-radius: 5px;
    background: linear-gradient(90deg, #d7b263, rgba(215,178,99,0));
  }
  .zh {
    max-width: 680px;
    margin: 0 0 32px;
    color: #aeb4c4;
    font-size: clamp(15px, 1.25vw, 18px);
    line-height: 1.9;
    letter-spacing: .01em;
  }
  .legal {
    display: flex;
    gap: 18px;
    align-items: flex-start;
    max-width: 680px;
    padding: 22px 24px;
    border: 1px solid #303748;
    border-left: 7px solid #d7b263;
    border-radius: 18px;
    background: rgba(255,255,255,.025);
  }
  .legal svg { width: 24px; height: 24px; flex: none; margin-top: 2px; color: #b99a5c; }
  .legal-title { margin-bottom: 8px; color: #939bad; font-size: clamp(14px, 1.1vw, 16px); letter-spacing: .08em; }
  .legal p { margin: 0; color: #d6ad60; font-size: clamp(15px, 1.25vw, 18px); line-height: 1.65; }
  .legal-link { color: inherit; text-decoration: underline; text-decoration-color: rgba(214,173,96,.82); text-underline-offset: 5px; transition: color .18s ease, text-decoration-color .18s ease; }
  .legal-link:hover, .legal-link:focus-visible { color: #f0c86f; text-decoration-color: #f0c86f; }
  .legal-link:focus-visible { outline: 2px solid rgba(240, 200, 111, .7); outline-offset: 4px; border-radius: 2px; }
  .apology { margin: 40px 0 36px; color: #aeb4c4; font-size: clamp(15px, 1.25vw, 18px); }
  .identity-cta {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    margin: 0 0 36px;
    padding: 12px 18px;
    border: 1px solid rgba(236, 190, 92, .9);
    border-radius: 12px;
    background: linear-gradient(135deg, #d7a94f, #a97922);
    color: #17130b;
    font-size: clamp(14px, 1.15vw, 16px);
    font-weight: 750;
    text-decoration: none;
    box-shadow: 0 10px 24px rgba(0, 0, 0, .25);
  }
  .identity-cta:hover, .identity-cta:focus-visible { background: #f0c86f; outline: 3px solid rgba(240, 200, 111, .35); outline-offset: 3px; }
  .divider { height: 1px; margin-bottom: 28px; background: #303748; }
  .en { max-width: 680px; margin: 0 0 32px; color: #777f92; font-size: clamp(13px, 1.1vw, 16px); line-height: 1.8; }
  .footer { display: flex; justify-content: space-between; gap: 18px; color: #626b7f; font-size: clamp(11px, .9vw, 13px); letter-spacing: .12em; }
  @media (max-width: 700px) {
    body { padding: 16px; }
    .card { min-height: calc(100vh - 32px); padding: 42px 24px 34px; border-radius: 20px; }
    .watermark { top: 24px; right: 2%; font-size: 130px; }
    .tag { padding: 9px 17px; font-size: 14px; }
    .tag svg { width: 20px; height: 20px; }
    h1 { margin-top: 42px; }
    .zh { line-height: 1.9; }
    .legal { gap: 14px; padding: 22px 18px; border-left-width: 5px; }
    .legal p { font-size: 16px; }
    .apology { margin: 42px 0; }
    .identity-cta { margin-bottom: 42px; padding: 12px 16px; font-size: 15px; }
    .footer { flex-direction: column; gap: 12px; letter-spacing: .1em; }
  }
</style>
</head>
<body>
  <main class="card" role="main" aria-labelledby="blocked-title">
    <div class="watermark" aria-hidden="true">451</div>
    <div class="tag">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M12 3 4.5 6v5.2c0 4.7 3.1 8.1 7.5 9.8 4.4-1.7 7.5-5.1 7.5-9.8V6L12 3Z"/><path d="m9.2 11.8 1.8 1.8 3.8-4"/></svg>
      <span>HTTP 451 · 依法限制访问</span>
    </div>
    <h1 id="blocked-title">本站点不向中国大陆地区提供服务</h1>
    <p class="zh">本站点遵循中华人民共和国《生成式人工智能服务管理暂行办法》及相关法律法规的要求，不面向中国大陆境内公众提供生成式人工智能服务，并已对来自中国大陆地区的网络访问予以限制。</p>
    <section class="legal" aria-label="法规依据">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true"><path d="M6 3.5h8l4 4V20.5H6z"/><path d="M14 3.5v4h4M9 12h6M9 15.5h6"/></svg>
      <div><div class="legal-title">法规依据 · 官方发布</div><p><a class="legal-link" href="https://www.cac.gov.cn/2023-07/13/c_1690898327029107.htm" target="_blank" rel="noopener noreferrer">《生成式人工智能服务管理暂行办法》（国家互联网信息办公室等七部门联合发布）</a></p></div>
    </section>
    <p class="apology">由此给您带来的不便，敬请谅解。</p>
    <a class="identity-cta" href="/api/access-policy/whitelist" aria-label="企业用户或教育用户申请当前 IP 白名单">企业用户/教育用户？点击此处加入 IP 白名单后，即可访问</a>
    <div class="divider"></div>
    <p class="en">In accordance with the Interim Measures for the Administration of Generative Artificial Intelligence Services and related laws and regulations, this service is not available to users in mainland China.</p>
    <footer class="footer"><span>ERROR 451 · UNAVAILABLE FOR LEGAL REASONS</span><span>ACCESS RESTRICTED</span></footer>
  </main>
</body>
</html>`
