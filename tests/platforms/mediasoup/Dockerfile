# mediasoup-demo server at a pinned commit (specs/010-mediasoup-platform
# research R11). The repository publishes no image and no release tags; the
# commit's package-lock.json pins mediasoup and every other dependency.
FROM node:22-trixie

ARG MEDIASOUP_DEMO_COMMIT=899109e4046b45da1623a3e4c65a742040479439

RUN git clone https://github.com/versatica/mediasoup-demo.git /demo \
	&& git -C /demo checkout --quiet "$MEDIASOUP_DEMO_COMMIT"

WORKDIR /demo/server
# npm ci runs the package's prepare script, which compiles the TypeScript
# into lib/. mediasoup downloads its prebuilt worker, which needs a recent
# glibc: bookworm's 2.36 is too old for it, trixie's runs it.
RUN npm ci && test -f lib/index.js

# config.mjs is mounted by docker-compose.yml.
CMD ["node", "lib/index.js"]
