# The encoders' own SVC example programs, for pre-encoding VP9 and AV1
# scalable content (specs/014-video-codec-comparison research R4). Build-time
# only: nothing here ships with the engine.
#
#   docker build -t webrtc-load-engine/svc-encoders scripts/svc-encoders
#
# vp9_spatial_svc_encoder: --inter-layer-pred 2 is "key frames only"
# (L3T3_KEY). svc_encoder_rtc: --layering-mode 10 is the KSVC pattern, 3
# spatial and 3 temporal layers with inter-layer prediction only on
# superframes whose base is key (L3T3_KEY). vpxdec and aomdec decode one
# spatial layer alone (--svc-decode-layer, --oppoint), for its VMAF.
FROM ubuntu:24.04 AS build

ARG LIBVPX_VERSION=v1.15.0
ARG LIBAOM_VERSION=v3.12.0

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates git build-essential cmake nasm yasm perl python3 \
    && rm -rf /var/lib/apt/lists/*

RUN git clone --depth 1 --branch "$LIBVPX_VERSION" https://chromium.googlesource.com/webm/libvpx /src/libvpx \
    && cd /src/libvpx \
    && ./configure --enable-vp9 --enable-examples --disable-unit-tests --disable-docs \
    && make -j"$(nproc)" \
    && install -m 755 examples/vp9_spatial_svc_encoder vpxdec /usr/local/bin/

RUN git clone --depth 1 --branch "$LIBAOM_VERSION" https://aomedia.googlesource.com/aom /src/aom \
    && cmake -S /src/aom -B /src/aom-build -DCMAKE_BUILD_TYPE=Release \
        -DENABLE_EXAMPLES=1 -DENABLE_TESTS=0 -DENABLE_DOCS=0 -DENABLE_TOOLS=0 \
        -DCONFIG_AV1_DECODER=1 \
    && cmake --build /src/aom-build -j"$(nproc)" --target svc_encoder_rtc aomdec \
    && install -m 755 /src/aom-build/examples/svc_encoder_rtc /src/aom-build/aomdec /usr/local/bin/

FROM ubuntu:24.04
COPY --from=build /usr/local/bin/vp9_spatial_svc_encoder /usr/local/bin/vpxdec \
    /usr/local/bin/svc_encoder_rtc /usr/local/bin/aomdec /usr/local/bin/
WORKDIR /work
