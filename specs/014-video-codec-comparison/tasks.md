---

description: "Task list for 014 video codec comparison"
---

# Tasks: Video Codec Comparison

**Input**: Design documents from `specs/014-video-codec-comparison/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/test-plan-codec.md](contracts/test-plan-codec.md), [contracts/report-codec.md](contracts/report-codec.md), [quickstart.md](quickstart.md)

**Tests**: Included. The plan and the constitution require them: write each test first and see it fail. Real-server tests stay behind `-tags live`.

**Organization**: the codec value, its test plan field, the content manifest and the new video track are foundational (Phase 2). US1 sends each codec as a single stream on the four servers; US2 makes every measure work for every codec; US3 adds layers in every codec; US4 gathers the remote runs.

**Conventions**:
- **Toolchain**: Go at `/usr/local/go/bin` (`export PATH=/usr/local/go/bin:$PATH`). Code stays go1.23-compatible. `go vet ./... && go test -race ./...` stays green at the end of each phase and needs no server.
- **Boundaries** (plan, constitution II and III):
  - no new module in `go.mod`;
  - codec knowledge (payloaders, parsers, descriptor, trailer carriers, content) stays in `src/media`, which imports no platform package;
  - each server's codec, simulcast and layer requests stay in its adapter;
  - servers' code is not changed; only `tests/platforms/mediasoup/config.mjs` and, if R17 says so, `tests/platforms/docker-jitsi-meet` settings;
  - the VP8 asset files (`src/media/assets/video*.ivf`) are not changed (FR-011).
- **Names** (used across tasks):
  - in `src/media/codec.go`: `type VideoCodec string` with `CodecVP8 = "vp8"`, `CodecVP9 = "vp9"`, `CodecAV1 = "av1"`, `CodecH264 = "h264"`; `ParseVideoCodec(s string) (VideoCodec, error)` (empty gives `CodecVP8`); `(VideoCodec).MimeType() string`; `(VideoCodec).FmtpLine() string` (`profile-id=0` for VP9, `level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f` for H.264, empty otherwise); `type Arrangement string` with `ArrangementSingle = "single"`, `ArrangementSimulcast = "simulcast"`, `ArrangementSVC = "svc"`; `(VideoCodec).Arrangement(layered bool) Arrangement`;
  - in `src/media/content.go`: `LoadVideo(codec VideoCodec, arr Arrangement, maxHeight int) ([]*Content, error)` (one `Content` per height for `single`/`simulcast`, one for `svc`); `LoadContent` and `LoadLayers` stay and call it with `CodecVP8`; `Content` gains `VideoCodec VideoCodec` and `Heights []int` (its `Codec` field stays the Pion capability);
  - in `src/media/manifest.go`: `ContentInfo{Codec VideoCodec; Arrangement Arrangement; Height, BitrateKbps int; VMAF, VMAFReference float64}` read from the embedded `assets/content.json`; `ContentBitrates(codec, arr, maxHeight) map[int]int`;
  - in `src/media/frame.go`: `FrameDesc{Start, End, Keyframe bool; Width, Height, SpatialID, TemporalID int}`; `type FrameParser interface { Parse(p *rtp.Packet) (FrameDesc, bool) }`; `NewFrameParser(codec VideoCodec, ddExtID uint8) FrameParser`;
  - in `src/media/dependency_descriptor.go`: `DependencyDescriptorURI = "https://aomediacodec.github.io/av1-rtp-spec/#dependency-descriptor-rtp-header-extension"`; `type DDStructure`, `DDSingleLayer`, `DDL3T3Key(widths, heights [3]int) DDStructure`; `WriteDD(d DD) []byte`; `ParseDD(b []byte, s *DDStructure) (DD, error)`;
  - in `src/execution/video_track.go`: `videoTrack` implementing `webrtc.TrackLocal` and `media.TrackWriter`; `newVideoTrack(codec media.VideoCodec, id, streamID, rid string) *videoTrack`; `errCodecMismatch`; `(*Peer).LocalTracks()` and `LocalLayerTracks()` return `sendTrack` (`interface{ webrtc.TrackLocal; WriteRTP(*rtp.Packet) error }`) instead of `*webrtc.TrackLocalStaticRTP`;
  - in `src/execution/metrics.go`: `stageMedia = "media"`; failure reason `codec_mismatch` with the text `codec_mismatch: sending negotiated VP8, plan asks for av1 (platform mediasoup)` ([contract](contracts/report-codec.md)).
- **Live runs**: local servers in `tests/platforms` first, one at a time; compared CPU only on the 013 remote machine `fetis@192.168.1.95`, alternated, same machine state. Stop only what you started. Never `pkill -f`: kill by PID. Never run `sfu-remote allow`, never pass `--external-firewall`.
- **A server that cannot do something** (R17): record it in `validation-report.md` with the server's reason and make the run fail with a named reason (FR-017). Do not patch the server. If it would change the spec, stop and tell the user.
- **Gates**: T002 (R17 facts) before any adapter task; T025 (browser, single stream) before US2; T043 (browser with trailer) before US3; T051 (browser, SVC) before the US3 adapter tasks. A gate that fails stops the work and goes to the user.
- **Commits**: only when the user asks.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story the task belongs to (US1 to US4)

---

## Phase 1: Setup

**Purpose**: Open the report, check the servers' facts and the content tools before code depends on them.

- [X] T001 Create `specs/014-video-codec-comparison/validation-report.md` with one section per success criterion (SC-001 to SC-009) and a "Server facts (R17)" section; record `git rev-parse HEAD`, `go vet ./... && go test -race ./...` and the size of `go build -o /tmp/wle ./src` (R15 baseline)
- [X] T002 Check the R17 facts on the local servers and record each answer, with the command and output, in `validation-report.md` "Server facts": (1) start `tests/platforms/docker-jitsi-meet`, read jicofo's `ENABLE_CODEC_*`/`codec` settings in its container and the codecs in a session-initiate (`go run ./cmd/jitsi-connect ... --log-xmpp`); (2) LiveKit's enabled codecs (`livekit.yaml`, `room.enabled_codecs` default) and whether it reads the dependency descriptor for AV1 and VP9 SVC (its source at the pinned tag); (3) mediasoup 3.30.1 `supportedRtpCapabilities` for AV1 and the dependency descriptor (`node -e` in the `mediasoup-demo` container); (4) Janus v1.4.3 VideoRoom `videocodec`, `vp9_profile`, `h264_profile`, AV1 SVC and H.264 simulcast support (its `conf` and source at the tag). Stop the servers you started
- [X] T003 [P] Create `scripts/svc-encoders/Dockerfile`: libvpx and libaom at pinned tags (record them in the file), building `vp9_spatial_svc_encoder` and `svc_encoder_rtc`; build it, run both on 30 raw I420 `testsrc2` frames at 640x360 (and 1280x720) with 3 spatial and 3 temporal layers in `L3T3_KEY` mode, and record in `validation-report.md` the exact command lines, output format (IVF of superframes / temporal units), the layers' sizes, and that every frame is shown at 30 fps (R4, R17.5)

---

## Phase 2: Foundational (the codec in the plan, content and the video track)

**Purpose**: A plan names a codec, the engine loads its content and sends it on a track that knows its codec. All stories depend on this phase.

- [X] T004 [P] Write failing tests in `src/media/codec_test.go`: `ParseVideoCodec` accepts `vp8`, `vp9`, `av1`, `h264` and empty (gives `vp8`); it refuses `VP8`, `vp10`, `h265` with an error naming "vp8, vp9, av1, h264"; `MimeType` gives `video/VP8`, `video/VP9`, `video/AV1`, `video/H264`; `FmtpLine` as in the Names; `Arrangement(false)` is `single` for all four, `Arrangement(true)` is `simulcast` for VP8 and H.264 and `svc` for VP9 and AV1
- [X] T005 [P] Implement `src/media/codec.go` with the names above
- [X] T006 [P] Write failing tests in `src/scenario/scenario_test.go`: `media.video_codec` is accepted with the four values; absent behaves as before (SC-007: an existing plan's JSON round-trips without the field); refused, with the field named and the four values listed, for an unknown value; refused with a profile other than `audio_video`; refused for `source_file` with `vp9` or `av1` and `simulcast` set (research R13); `simulcast.max_height` still "360 or 720" for every codec ([contract](contracts/test-plan-codec.md))
- [X] T007 Add `VideoCodec string \`json:"video_codec,omitempty"\`` to `MediaConfig` in `src/scenario/scenario.go` with its validation, using `media.ParseVideoCodec`; a method `(MediaConfig).Codec() media.VideoCodec` and `(MediaConfig).Arrangement() media.Arrangement`
- [X] T008 [P] Write failing tests in `src/media/manifest_test.go`: the embedded `content.json` parses; it has a VP8 entry per height 90, 180, 360, 720 with `vmaf == vmaf_reference`; `ContentBitrates(CodecVP8, single, 360)` gives `{360: 500}`; an entry missing for a codec the loader is asked for is an error naming codec, arrangement and height
- [X] T009 Write `src/media/assets/content.json` with today's VP8 entries only (bitrates from `scripts/gen-media.sh`; `vmaf` and `vmaf_reference` filled by T016), and implement `src/media/manifest.go` (`//go:embed assets/content.json`)
- [X] T010 Write failing tests in `src/media/content_test.go`: `LoadVideo(CodecVP8, single, 360)` gives exactly what `LoadContent(KindVideo)` gives today (same frames, bytes and timestamps); `LoadVideo(CodecVP8, simulcast, 720)` equals `LoadLayers(720)`; each `Content` carries `Codec` and `Heights`; an unknown codec or a missing file is an error naming it
- [X] T011 Refactor `src/media/content.go` to `LoadVideo` (`Content.Codec` stays the capability, set from `VideoCodec.MimeType`/`FmtpLine`), keep `LoadContent`/`LoadLayers` as VP8 wrappers, and update the callers in `src/execution/peer.go` and tests; all existing tests stay green
- [X] T012 Write failing tests in `src/execution/video_track_test.go`, with two Pion PeerConnections in-process (as `peer_test.go`): a `videoTrack` for VP9 bound to an answer that negotiated VP9 rewrites SSRC and payload type of written packets; bound to an answer offering only VP8, `Bind` returns `errCodecMismatch` naming "VP8"; with the dependency descriptor negotiated, the extension ID in each written packet is the binding's ID, and packets without it carry none; `WriteRTP` to two bindings with different IDs does not change the caller's packet (shared packets, research R7)
- [X] T013 Implement `src/execution/video_track.go` (`Bind`, `Unbind`, `ID`, `RID`, `StreamID`, `Kind`, `Codec`, `WriteRTP`; per binding SSRC, payload type, descriptor ID, rewrite on a copy of the header only) and add `stageMedia` to `src/execution/metrics.go`
- [X] T014 Switch video in `src/execution/peer.go` from `TrackLocalStaticRTP` to `videoTrack` (single and layer tracks), change `LocalTracks`/`LocalLayerTracks` to return `sendTrack`, update `src/execution/engine.go`; a `Bind` error with `errCodecMismatch` fails the bot at `stageMedia` with the `codec_mismatch` text; VP8 runs unchanged (`go test -race ./src/execution/ ./tests/...` without `live` green)

**Checkpoint**: VP8 sends through the new track with the same results; a plan can name a codec.

---

## Phase 3: User Story 1 - Choose the video codec of a run (Priority: P1) 🎯 MVP

**Goal**: Each of the four codecs is sent as a single stream on the four servers, checked on both ends, and named in the report.

**Independent Test**: On the local LiveKit server, 10 bots, `audio_video`, 1 minute, once per codec: each run passes and each report says the codec sent and received (quickstart §3).

### Content

- [X] T015 [US1] Extend `scripts/gen-media.sh`: for VP9 (`libvpx-vp9`, profile 0, CBR, `-auto-alt-ref 0 -lag-in-frames 0`, IVF), AV1 (`libaom-av1`, `-usage realtime`, CBR, `-lag-in-frames 0`, IVF) and H.264 (`libx264`, `-profile:v baseline -level 3.1`, `-bf 0 -sc_threshold 0`, `-x264-params repeat-headers=1:aud=1`, Annex B `.h264`), at heights 90, 180, 360, 720, `testsrc2`, 30 fps, 10 s, `-g 30` keyframes at the same frames as VP8; VMAF (`libvmaf`, default model) of each against the raw source at that size; the VP8 reference VMAF of each existing `video*.ivf` (not re-encoded); bisection of each codec's CBR bitrate until "`abs(vmaf - vmaf_reference) <= 1.0`" (SC-009), at most 10 encodes, else fail naming codec and height; write the files as `src/media/assets/{vp9,av1}_<h>.ivf`, `h264_<h>.h264` and every entry of `content.json`
- [X] T016 [US1] Run `scripts/gen-media.sh`; check `git diff --stat src/media/assets/video*.ivf` shows nothing, every `content.json` entry meets the VMAF rule, every file has 300 frames and keyframes at 0, 30, …; record the bitrates, VMAF values and the binary size growth in `validation-report.md` (SC-009, R15)
- [X] T017 [P] [US1] Write failing tests in `src/media/content_test.go`: `LoadVideo` for VP9 and AV1 at each height gives 300 frames, keyframes every 30 (VP9: frame header not P; AV1: temporal unit with a sequence header and a key frame), no temporal delimiter OBU kept in AV1 frames; for H.264, 300 access units split at each AUD, keyframe access units start with SPS, PPS and an IDR, the AUD is dropped; all codecs' frames have the same timestamps as VP8's
- [X] T018 [US1] Implement codec-aware loading in `src/media/content.go` (Pion `ivfreader` for VP9 and AV1, `h264reader` grouped into access units for H.264); AV1 frames stored as OBUs without temporal delimiters, size fields kept for the payloader to strip

### Packetization

- [X] T019 [P] [US1] Write failing tests in `src/media/vp9_test.go`: the single-layer payloader (RFC 9628 flexible mode, `I=1` with a 15-bit picture ID incremented per picture, `P=0` on keyframes, `B`/`E` on the first/last packet, `V=1` with the scalability structure giving 640x360 on keyframes) splits each real VP9 frame from `LoadVideo` into packets of at most the MTU; Pion's `codecs.VP9Packet.Unmarshal` reads back the same picture ID, flags and payload bytes, and their concatenation equals the frame
- [X] T020 [P] [US1] Write failing tests in `src/media/av1_test.go`: the payloader writes the aggregation header (Z, Y, W, N per the AV1 RTP spec; `N=1` on the first packet of a keyframe), OBUs without `obu_has_size_field`, no temporal delimiter; Pion's `codecs.AV1Packet` plus `obu` reassembly gives back the frame's OBUs; a frame larger than the MTU is fragmented with Z/Y set
- [X] T021 [P] [US1] Write failing tests in `src/media/dependency_descriptor_test.go`: `WriteDD` with `DDSingleLayer` and `attached structure` on a keyframe's first packet, then without on others, round-trips through `ParseDD`; the bytes match the single-layer example of the AV1 RTP specification; start/end-of-frame bits and frame number round-trip over 65536 frames
- [X] T022 [US1] Implement `src/media/vp9.go` (payloader, single layer for now), `src/media/av1.go` (payloader) and `src/media/dependency_descriptor.go` (writer, reader, `DDSingleLayer`)
- [X] T023 [US1] Write failing tests in `src/media/source_test.go`, then change `src/media/source.go`: `packetize` picks the payloader by `Content.Codec` (VP8 today's, VP9 and AV1 the new ones, H.264 `codecs.H264Payloader`), the marker is on the last packet of each frame, AV1 packets carry the descriptor as header extension (ID 0 placeholder, rewritten per binding by T013), packetization still happens once per frame for all subscribers; VP8 packets are byte-identical to before
- [X] T024 [US1] Register the dependency descriptor header extension for video on both PeerConnections in `src/execution/pion.go`, and make `src/execution/peer.go` load content with `LoadVideo(plan codec, single, …)` and create the `videoTrack` with that codec

### Browser gate (single stream)

- [X] T025 [US1] Gate (R16, SC-004, without the trailer: disable it for non-VP8 codecs until T042): start `tests/platforms/janus` with a VideoRoom per codec (`videocodec`, `vp9_profile: "0"`, `h264_profile: "42e01f"`), run 2 bots per codec with a long hold through a scratch program or `go test` helper, and ask the user to join each room in Chrome and Firefox; record per codec and browser in `validation-report.md` whether the moving pattern shows without artefacts. A failure stops here and goes to the user

### Codec checks

- [X] T026 [P] [US1] Write failing tests in `src/execution/peer_test.go`: a received video track whose codec is not the plan's fails the bot at `stageMedia` with `codec_mismatch: receiving <codec>, plan asks for <plan> (platform <p>)`; the bot records the codecs of the video tracks it received for the report
- [X] T027 [US1] Implement the receive check in `src/execution/peer.go` `onTrack` and the per-bot received codecs in the bot's result (`src/execution/engine.go`, `src/scenario/result.go`)

### Platforms (after T002)

- [X] T028 [P] [US1] Jitsi, tests first in `src/platform/jitsi/adapter_test.go` and `jingle_sdp_test.go` with the fake server: the answer to a session-initiate offering the four codecs keeps them; the bot's presence puts the plan's codec first in `jitsi_participant_codecList` (if T002 shows jicofo reads it); a session-initiate without the plan's codec makes the bot fail with `codec_mismatch` naming the codecs offered. Implement in `src/platform/jitsi/`; set the codec preference on the sending transceiver (`SetCodecPreferences`) so the plan's codec is negotiated
- [X] T029 [P] [US1] LiveKit, tests first in `src/platform/livekit/proto_test.go` and `adapter_test.go`: `AddTrackRequest` carries the plan's codec MIME in `simulcast_codecs` (field numbers from livekit/protocol v1.52.1, cite them in a comment as 008 does) and `width`/`height`; the publish offer lists the plan's codec first. Implement in `src/platform/livekit/proto.go` and `adapter.go`
- [X] T030 [P] [US1] mediasoup, tests first in `src/platform/mediasoup/sdp_test.go` and `adapter_test.go`: the bot's `rtpCapabilities` list the four codecs (VP9 `profile-id=0`, H.264 `42e01f` mode 1) and the dependency descriptor; `produce` for video sends the plan's codec's `rtpParameters` (payload type, fmtp, the descriptor for AV1); a router without the codec makes the bot fail with `codec_mismatch` naming the router's codecs. Implement in `src/platform/mediasoup/`; add VP9 `profile-id: 0` and `AV1` to the router codecs in `tests/platforms/mediasoup/config.mjs` (if T002 shows 3.30.1 supports AV1; otherwise record it and leave AV1 out)
- [X] T031 [P] [US1] Janus, tests first in `src/platform/janus/messages_test.go` and `adapter_test.go`: `create` sends `videocodec` set to the plan's codec with `vp9_profile: "0"` or `h264_profile: "42e01f"`, `publish` names it, the VP8 run sends exactly today's messages; implement in `src/platform/janus/adapter.go` (lines with `"videocodec": "vp8"`) and `messages.go`
- [X] T032 [US1] If T002 found a codec disabled by default in jicofo, enable it in `tests/platforms/docker-jitsi-meet` (`env.example` and `setup.sh`, also `docker-compose.remote.yml` for 013) and document it in that folder's `README.md`

### Report

- [X] T033 [US1] Write failing tests in `src/report/run_test.go` and `markdown_test.go` (golden files in `src/report/testdata`), then implement in `src/scenario/quality.go`, `src/report/run.go` and `markdown.go`: `quality.video.codec` with `sent`, `arrangement`, `content_bitrate_kbps` (from `media.ContentBitrates`), `received` (codec → bots), `mismatch` (`{"bot", "direction": "sent"|"received", "codec"}`), `layers_not_forwarded` (false until T058); per bot `video_codec_received`; `FormatVersion` 2; Markdown rows "Video codec" and "Video content bitrate" after "Outcome", and with a mismatch the first row reads `vp9 asked · 3 bots on another codec: see Video` ([contract](contracts/report-codec.md)); a VP8 report differs from before only by these fields and the version

### Tools and live runs

- [X] T034 [P] [US1] Add `--video-codec` to `cmd/jitsi-connect/main.go`, and `<PLATFORM>_VIDEO_CODEC` to `tests/live/{jitsi,livekit,mediasoup,janus}_live_test.go` (documented at the top of each file); each live test checks that the report's codec `sent` and `received` are the asked codec for every bot
- [X] T035 [US1] Live, one server at a time locally: 10 bots, `audio_video`, 1 minute, per codec on LiveKit, mediasoup, Janus and Jitsi (16 runs, quickstart §3); record each outcome in `validation-report.md` SC-001 and SC-002; a server that refuses a codec is recorded with its reason; then the mismatch check: one mediasoup run with AV1 removed from `config.mjs` fails with `codec_mismatch` and reports no AV1 figures (restore the file after)
- [X] T036 [US1] Media source files (FR-010): tests first in `src/media/file_content_test.go`, then `src/media/file_content.go` transcodes a `source_file` into the run's codec with T015's settings at the bitrates `content.json` gives per height (single and simulcast; SVC is refused at creation by T007); check one H.264 run with a file through `jitsi-connect --media-file`

**Checkpoint**: Every codec runs as a single stream on every server, or the server's reason is recorded; reports name the codec. MVP.

---

## Phase 4: User Story 2 - The same measures for every codec (Priority: P2)

**Goal**: Freezes, frame rate, resolution and latency for every codec, as for VP8.

**Independent Test**: 10 bots, 1 minute, each codec on one server: each report gives freezes, frame rate, resolution and video latency, or a named reason, never a zero (quickstart §4).

- [X] T037 [P] [US2] Write failing tests in `src/media/frame_test.go`, `vp9_test.go`, `av1_test.go`, `h264_test.go` with packets from the US1 payloaders over the real content: each codec's `FrameParser` gives `Start`/`End`/`Keyframe` on the right packets; VP9 `Width`/`Height` from the scalability structure; AV1 from the descriptor structure or the sequence header; H.264 from the SPS (Exp-Golomb read of `pic_width_in_mbs_minus1`, `pic_height_in_map_units_minus1`, frame cropping), 640x360 for the 360 content; VP8's parser gives the same results as `parseVP8` today
- [X] T038 [US2] Implement `src/media/frame.go`, the VP9 and AV1 receive parsers in `vp9.go` and `av1.go`, `src/media/h264.go` (NAL types, STAP-A, FU-A, SPS size), and a VP8 `FrameParser` wrapping `parseVP8`
- [X] T039 [US2] Write failing tests in `src/media/video_quality_test.go`, then make `src/media/video_quality.go` codec-neutral: `VideoQuality` takes `FrameDesc` (`UpdateRTP(p *rtp.Packet, arrival time.Time)` with the parser given at creation); freezes, frame rate, resolution and keyframe requests behave as today for VP8 (all existing tests green) and for each codec on the same sequences
- [X] T040 [US2] Measure video for every codec in `src/execution/peer.go` (the VP8-only test at line 419), creating the parser with the binding's descriptor extension ID
- [X] T041 [P] [US2] Write failing tests in `src/media/frame_trailer_test.go`: per codec carrier ([data-model](data-model.md) "Frame trailer carrier"): VP8 unchanged; VP9 trailer then `0x00`, the layer frame never ends in `0xC0`–`0xDF`; AV1 an `OBU_PADDING` OBU, last in the last packet, the trailer its last 6 bytes; H.264 a user data unregistered SEI with the project UUID in its own last packet, emulation prevention bytes inserted and removed, a payload containing `00 00 01` read back exactly; the reader of each codec finds the frame number and rejects another scenario's tag
- [X] T042 [US2] Implement the carriers in `src/media/frame_trailer.go`, writing them in `source.go` before packetization and reading them in `video_quality.go` `measureLatency` by codec; enable the trailer for every codec (removing T025's switch)
- [X] T043 [US2] Gate (R16, SC-004 with the trailer): repeat T025 with the trailer on, in Chrome and Firefox; record the result in `validation-report.md`. A codec that shows artefacts stops here: revise its carrier (research R9) with the user
- [X] T044 [US2] Live: rerun T035's 16 single-stream runs and check each report gives freezes, frame rate, resolution and latency, or a named reason that the VP8 run on that server also gives (Janus audio stays `frame_not_identified`); record SC-003 in `validation-report.md`

**Checkpoint**: US1 and US2 answer the user's question for single streams.

---

## Phase 5: User Story 3 - Layered video in every codec (Priority: P3)

**Goal**: VP8 and H.264 simulcast, VP9 and AV1 `L3T3_KEY` SVC, with requested heights and the receive cap, on every server that forwards them.

**Independent Test**: On one server, 10 bots, 1 minute, layers and a receive cap, some bots asking for 180: each run passes, those bots receive 180 (quickstart §5).

### SVC content and sending

- [X] T045 [US3] Add an `svc` mode to `scripts/gen-media.sh`: builds `scripts/svc-encoders`, feeds raw I420 `testsrc2` frames to the T003 command lines for max heights 360 (90/180/360) and 720 (180/360/720), `L3T3_KEY`, keyframes every 30 frames; bisects each spatial layer's bitrate to the VMAF of the VP8 simulcast layer of that height (rule "`abs(vmaf - vmaf_reference) <= 1.0`", VMAF of each layer decoded alone at its size); writes `vp9_svc_<max>.ivf`, `av1_svc_<max>.ivf` and their `content.json` entries; run it and record the results in `validation-report.md` SC-009
- [X] T046 [P] [US3] Write failing tests in `src/media/content_test.go`: `LoadVideo(CodecVP9, svc, 720)` splits each superframe by its index into 3 layer frames with spatial IDs 0, 1, 2 and the right sizes; AV1 temporal units split by the OBU extension header's spatial ID; temporal IDs follow the T1-T2-T1-T0 pattern of `L3T3`; 300 pictures, keyframes at the same pictures as VP8; `Content.Heights` lists the three heights
- [X] T047 [P] [US3] Write failing tests in `src/media/vp9_test.go` and `dependency_descriptor_test.go`: the VP9 payloader writes per layer frame `L=1` with TID, U, SID, D, reference differences, `B`/`E` per layer frame, the scalability structure with the three resolutions on keyframes, the marker only on the last packet of the picture's highest layer; `DDL3T3Key` matches the `L3T3_KEY` example of the AV1 RTP specification (templates, decode targets, chains), and per packet the template ID follows the layer frame's spatial and temporal IDs
- [X] T048 [US3] Implement SVC loading in `src/media/content.go`, layers in `vp9.go`, `DDL3T3Key` in `dependency_descriptor.go`, and in `source.go` one stream for SVC: all layer frames of a picture with one timestamp, the 012 trailer in each layer frame (research R9), VP9 SVC packets also carry the descriptor
- [X] T049 [US3] H.264 simulcast: tests first in `src/media/source_test.go`, then the layered source accepts H.264 content per height as it does VP8 (T015 already wrote the heights); `src/execution/peer.go` creates three RID `videoTrack`s for `simulcast` and one for `svc`, by `MediaConfig.Arrangement()`
- [X] T050 [P] [US3] Write failing tests in `src/media/video_quality_test.go`: on an SVC stream the received picture's height is the highest spatial layer seen for its timestamp; dropping the upper layers mid-run changes the height and is not counted as a freeze; latency is still measured from the lowest layer's trailer; then implement in `src/media/video_quality.go`
- [X] T051 [US3] Gate (R16, SC-004, SVC): on the local Janus (and LiveKit if T002 says it forwards SVC), each of VP9 and AV1 SVC and H.264 simulcast shows in Chrome and Firefox without artefacts, including when the browser switches layers; record in `validation-report.md`. A failure stops here

### Platforms: simulcast, requested heights and SVC (VP8 first, then the other codecs)

- [X] T052 [P] [US3] LiveKit, tests first in `src/platform/livekit/config_test.go`, `proto_test.go`, `subscriptions_test.go`: `config.go` accepts `simulcast` and `requested_heights` (remove the refusals at line 155); `AddTrackRequest` carries `layers` (one `VideoLayer` per height, quality LOW/MEDIUM/HIGH, width, height, bitrate from `content.json`) and for SVC the codec's `scalability_mode` `L3T3_KEY`; simulcast by RIDs `q`, `h`, `f`; requested heights by `UpdateTrackSettings` (width, height) per subscribed track; implement in `src/platform/livekit/`
- [X] T053 [P] [US3] mediasoup, tests first in `src/platform/mediasoup/config_test.go`, `sdp_test.go`, `consumers_test.go`: `config.go` accepts both (line 152); `produce` sends three encodings with RIDs for simulcast, one encoding with `scalabilityMode: "L3T3_KEY"` for SVC; requested heights by the demo's `setConsumerPreferredLayers` (spatial layer index from the height); the bot's receive SDP handles the consumer's layered `rtpParameters`; implement in `src/platform/mediasoup/`
- [X] T054 [P] [US3] Janus, tests first in `src/platform/janus/config_test.go`, `messages_test.go`, `streams_test.go`: `config.go` accepts both (line 170); simulcast by RIDs in the publish offer; SVC for VP9 and AV1 as one stream; requested heights by `configure` on the subscriber handle, `substream` for simulcast and `spatial_layer` (with `temporal_layer` 2) for SVC; implement in `src/platform/janus/`
- [X] T055 [P] [US3] Jitsi, tests first in `src/platform/jitsi/sources_test.go` and `json_sources_test.go`: for SVC the bot announces one video source with its RTX source and no SIM group, with `videoType` and the codec; simulcast in H.264 as VP8 today; requested heights through `ReceiverVideoConstraints` unchanged; implement in `src/platform/jitsi/sources.go`
- [X] T056 [US3] Add `<PLATFORM>_SIMULCAST`, `<PLATFORM>_REQUESTED_HEIGHTS` and `<PLATFORM>_MAX_RECEIVED` to `tests/live/{livekit,mediasoup,janus}_live_test.go` where missing, as the Jitsi live test has them; each checks the heights received against the requests (SC-008: at least 95% of measured time)

### Layers not forwarded and live runs

- [X] T057 [P] [US3] Write failing tests in `src/scenario/quality_test.go`: a layered run where no bot received a height above the lowest layer sets `layers_not_forwarded`, and the Markdown Video section says `The server forwarded only the lowest layer: layered result not available.`; a single-stream run never sets it
- [X] T058 [US3] Implement `layers_not_forwarded` in `src/scenario/quality.go`, `src/report/run.go` and `markdown.go`, naming the codec and the platform
- [X] T059 [US3] Live, locally, one server at a time: VP8 simulcast first on LiveKit, mediasoup and Janus (new for them), then each codec layered, 10 bots, 1 minute, `max_height: 720`, `max_received_streams: 4`, `requested_heights: 180:0.5,720:0.5` (quickstart §5); record per codec and server in `validation-report.md` SC-008 the share of time at the requested height, or `layers_not_forwarded` with the server's reason (FR-016, FR-017)

**Checkpoint**: Every codec layered on every server that forwards it; the others named.

---

## Phase 6: User Story 4 - The codecs side by side (Priority: P4)

**Goal**: One table per server in the validation report, from runs on the remote machine.

**Independent Test**: For one server, the validation report gives per codec at 10 bots server CPU and memory, MOS, freezes, latency and bitrate, with bot count, duration and receive cap.

- [X] T060 [US4] On `fetis@192.168.1.95`, one server at a time with `sfu-remote deploy` (the redeployed overlay includes T030/T032 changes) and the settings file loaded (`DOCKER_HOST=ssh://`): two VP8 runs (run-to-run variation), then one run per codec, single stream, 10 bots, 1 minute, alternated, same machine state; then the layered runs as T059; `sfu-remote remove` after each server. Keep the report files' paths
- [X] T061 [US4] Write `validation-report.md` SC-005: one table per server with codec, arrangement, content bitrate per height, server CPU and memory, MOS, freezes, frame rate, latency, from T060's reports, the bot count, duration, receive cap, date and machine above each table; say for each codec whether its server CPU differs from VP8's by more than the two VP8 runs differ; state that `testsrc2` is synthetic (research R3 limit) and any condition that differs between runs (US4 scenario 2)

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T062 [P] Add per-codec cases to `src/media/source_bench_test.go` and `src/media/quality_bench_test.go` (packetize and parse per codec, single and layered); record the results in `validation-report.md` SC-006
- [X] T063 Engine CPU at 100 bots per codec against VP8 on one local server (the engine's own `load_machine` figures), alternated; record per bot CPU and the difference in `validation-report.md` SC-006 (FR-012: within 10% or stated)
- [X] T064 SC-007: run the existing VP8 live tests without `<PLATFORM>_VIDEO_CODEC` on the four local servers and compare with the 012/013 figures in their validation reports; record it in `validation-report.md`
- [X] T065 [P] Document the codec settings in `tests/platforms/{mediasoup,docker-jitsi-meet,janus,livekit}/README.md` (what each server accepts, from T002 and T035), and FR-017 gaps if any
- [X] T066 [P] Update `CLAUDE.md` (Commands: a `*_VIDEO_CODEC` example and `jitsi-connect --video-codec`; Recent Changes: 014 in one entry, naming the trailer carriers and any server limit found)
- [X] T067 Run quickstart.md end to end where not already covered, then `gofmt -l .`, `go vet ./... && go test -race ./...`, and finish `validation-report.md` with a "Not met or not checked" section naming every criterion not fully met, as 013 did

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none. T002 blocks every platform task (T028–T032, T052–T055); T003 blocks T045.
- **Foundational (Phase 2)**: after T001. Blocks all stories.
- **US1 (Phase 3)**: after Phase 2. T015–T016 before T017; T022–T024 before T025; gate T025 before Phase 4.
- **US2 (Phase 4)**: after US1 (parsers are tested on US1's packets; live runs need the platforms). Gate T043 before Phase 5.
- **US3 (Phase 5)**: after US2 (layer changes are measured with US2's parsers and trailers). Gate T051 before T052–T055.
- **US4 (Phase 6)**: after US2 for the single-stream tables, after US3 for the layered ones.
- **Polish (Phase 7)**: T062 after T048; T063, T064 after US2; the rest at the end.

### Within Each Story

- Tests first, seen failing, then the code.
- Content before loaders, loaders before payloaders, payloaders before the source, the source before platforms.
- `src/media` before `src/execution` before `src/platform`.

### Parallel Opportunities

- T003 with T001–T002.
- T004, T006, T008 (different packages).
- T017, T019, T020, T021 once T016 has written the content (different files).
- T026 alongside T028–T031; the four adapter tasks are independent of each other.
- T037 and T041 (parsers and trailer tests).
- T046, T047, T050 (different files), then T052–T055 (one adapter each).
- T062, T065, T066.

---

## Parallel Example: User Story 1

```bash
# After T016, in parallel:
Task: "T017 content loading tests in src/media/content_test.go"
Task: "T019 VP9 payloader tests in src/media/vp9_test.go"
Task: "T020 AV1 payloader tests in src/media/av1_test.go"
Task: "T021 dependency descriptor tests in src/media/dependency_descriptor_test.go"

# After T002 and T025, in parallel:
Task: "T028 Jitsi codec in src/platform/jitsi/"
Task: "T029 LiveKit codec in src/platform/livekit/"
Task: "T030 mediasoup codec in src/platform/mediasoup/"
Task: "T031 Janus codec in src/platform/janus/"
```

## Parallel Example: User Story 3

```bash
# After T051, in parallel:
Task: "T052 LiveKit layers in src/platform/livekit/"
Task: "T053 mediasoup layers in src/platform/mediasoup/"
Task: "T054 Janus layers in src/platform/janus/"
Task: "T055 Jitsi SVC sources in src/platform/jitsi/sources.go"
```

---

## Implementation Strategy

### MVP First (User Story 1)

1. Phase 1, with T002's facts before planning adapter work in detail.
2. Phase 2: VP8 through the new track, unchanged.
3. Phase 3: content, payloaders, gate, platforms, report.
4. **Stop and validate**: 16 single-stream runs (T035). Server CPU per codec can already be read from the reports.

### Incremental Delivery

1. US1: codec chosen and checked → first comparison by server CPU.
2. US2: every measure for every codec → complete single-stream comparison.
3. US3: layers in every codec, new on LiveKit, mediasoup and Janus.
4. US4: remote runs and the tables.

Each gate (T025, T043, T051) is a stop: a browser that does not decode the bots' media invalidates what comes after.
