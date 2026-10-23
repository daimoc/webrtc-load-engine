# Local Janus server for 011 (specs/011-janus-platform/research.md R10).
# Janus has no official image: build a pinned tag with Debian's libraries.
FROM debian:bookworm-slim AS build
ARG JANUS_VERSION=v1.4.3
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates git build-essential autoconf automake libtool pkg-config gengetopt \
        libnice-dev libsrtp2-dev libjansson-dev libconfig-dev libssl-dev libglib2.0-dev \
        libcurl4-openssl-dev libwebsockets-dev zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*
RUN git clone --depth 1 --branch "$JANUS_VERSION" https://github.com/meetecho/janus-gateway.git /src
WORKDIR /src
# Only what the bots and the demo page use: VideoRoom over WebSocket.
RUN ./autogen.sh \
    && ./configure --prefix=/opt/janus \
        --disable-all-plugins --enable-plugin-videoroom \
        --disable-all-transports --enable-websockets \
        --disable-all-handlers --disable-all-loggers \
        --disable-data-channels --disable-docs \
    && make -j"$(nproc)" && make install

# The demos, served by nginx in their own container (docker-compose.yml).
FROM nginx:1.27-alpine AS demos
COPY --from=build /src/html /usr/share/nginx/html
COPY demos.conf /etc/nginx/conf.d/default.conf
# The demos talk to Janus over WebSocket (the HTTP transport is not built).
COPY demos-settings.js /usr/share/nginx/html/demos/settings.js

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
        libnice10 libsrtp2-1 libjansson4 libconfig9 libssl3 libglib2.0-0 \
        libcurl4 libwebsockets17 zlib1g \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /opt/janus /opt/janus
ENV JANUS_DEBUG_LEVEL=3 JANUS_ICE_ENFORCE_LIST=
CMD ["sh", "-c", "exec /opt/janus/bin/janus --configs-folder=/opt/janus/etc/janus --debug-level=${JANUS_DEBUG_LEVEL} ${JANUS_ICE_ENFORCE_LIST:+--ice-enforce-list=$JANUS_ICE_ENFORCE_LIST}"]
