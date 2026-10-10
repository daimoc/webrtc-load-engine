# Data Model: Synthetic Media Sending and Media Metrics

## Test plan: `media` (new, optional)

Part of `scenario.TestPlan` ([contracts/test-plan-media.yaml](contracts/test-plan-media.yaml)).

| Field | Type | Default | Validation |
|-------|------|---------|------------|
| `profile` | string | `receive_only` | one of `receive_only`, `audio`, `audio_video` |
| `stall_timeout_seconds` | integer | 10 | > 0 |
| `max_received_streams` | integer | 0 (no cap) | ≥ 0; per media kind |

A missing `media` object is equivalent to `{"profile": "receive_only"}`
(FR-002). Validation errors are returned when the scenario is created.

Profile → media kinds each bot sends:

| Profile | audio | video |
|---------|-------|-------|
| `receive_only` | no | no |
| `audio` | yes | no |
| `audio_video` | yes | yes |

## Media kind

`audio` or `video`. It is the only media vocabulary shared between the media
layer and the adapters (`SignalingHandler.LocalMediaKinds()`), and the `kind`
label of the media metrics.

## Media Content (package `media`, read-only)

Loaded once per process from the embedded files.

| Field | Description |
|-------|-------------|
| `kind` | audio or video |
| `codec` | `audio/opus` (48 kHz, 2 channels in SDP) or `video/VP8` (90 kHz) |
| `frames` | ordered list of encoded frames (payload, duration) |
| `loopDuration` | sum of frame durations |

## Media Source (package `media`, one per scenario and kind)

Created by the engine when the profile sends that kind; stopped when the
scenario ends.

| Field | Description |
|-------|-------------|
| `content` | the Media Content it loops |
| `packetizer` | RTP packetizer; sequence numbers and timestamps grow across loops |
| `tracks` | subscribed bot tracks (set, guarded by a mutex) |
| `start` | reference time; frame *i* of loop *k* is due at `start + k·loopDuration + offset(i)` |

Lifecycle: `New` → `Run(ctx)` (one goroutine) → stopped with the context.
`Subscribe(track)` / `Unsubscribe(track)` at any time.

## Peer media state (package `execution`, per peer)

| Field | Description |
|-------|-------------|
| `localTracks` | one track per sent kind, added before the first offer |
| `inbound` | per remote stream (SSRC): its kind and `InboundStats` |
| `lastReceived[kind]` | time of the last packet of that kind (stall detection), from the streams' stats |
| `offered[kind]` | remote sources of that kind in the last applied offer |
| `offeredSSRCs` | SSRCs of the last applied offer; Pion buffers for other SSRCs drop their packets (R12) |
| `rttSamples[kind]` | round trips from receiver reports, drained by the collector |

Stall state (`stalled[kind]`) is kept by the scenario's collector, per peer.

Media state never changes the peer lifecycle (FR-014); it is only read
while the peer is `established`.

## InboundStats (package `media`, per remote stream)

| Field | Description |
|-------|-------------|
| `packets`, `bytes` | received payload counters |
| `baseSeq`, `maxSeq`, `cycles` | extended highest sequence number (RFC 3550 A.1) |
| `expected − packets` | cumulative lost packets |
| `jitter` | RFC 3550 interarrival jitter, in timestamp units, converted to seconds with the clock rate |

## Jitsi session source state (package `jitsi`, per Jingle session)

| Field | Description |
|-------|-------------|
| `remote` | the current remote description: Jingle contents with their sources and groups |
| `sections` | ordered per-source SDP sections: content name, mid, remote source (none once removed) |
| `waiting` | per kind, remote sources beyond the receive cap, oldest first |
| `limit` | receive cap per kind (0 = none), from `ReceiveLimit()` |
| `version` | `o=` session version, incremented on each renegotiation |
| `local sources` | names `<nick>-a0` / `<nick>-v0`, owner `<room>/<nick>` |

Transitions on a `source-add`, a `source-remove` or the unavailable presence
of a source's owner: update `remote`, append a section for each new source
while its kind is below the cap (queue it otherwise), promote the oldest
waiting source of a kind when one of its active sources is removed,
make the sections of removed sources inactive (sections are never reused or
dropped, so mids and order stay stable), increment `version`, then call
`Renegotiate`.
