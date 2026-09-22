# Build in a dedicated build environment, never on production.
FROM node:22.16.0-bookworm-slim
COPY --from=oven/bun:1.4.2 /usr/local/bin/bun /usr/local/bin/bun
WORKDIR /renderer
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile --production
ENV PLAYWRIGHT_BROWSERS_PATH=/opt/playwright
RUN node node_modules/playwright/cli.js install --with-deps chromium
RUN apt-get update && apt-get install -y --no-install-recommends fonts-dejavu-core=2.37-6 && rm -rf /var/lib/apt/lists/*
RUN mkdir /fonts && cp /usr/share/fonts/truetype/dejavu/*.ttf /fonts/
RUN groupadd --gid 65532 renderer && useradd --uid 65532 --gid 65532 --no-create-home --home-dir /tmp/renderer-home renderer
COPY server.mjs render.mjs animation.mjs animation-child.mjs isolation.mjs ./
USER 65532:65532
ENV RENDERER_SOCKET=/run/capability/renderer.sock
ENV HOME=/tmp/renderer-home
CMD ["node", "server.mjs"]
