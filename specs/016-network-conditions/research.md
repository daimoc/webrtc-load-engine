# Research: Network Conditions per Bot (016)

Facts read in the code on 2026-11-15 (Pion interceptor v0.1.29, webrtc v3.3.6), and decisions.

## R1. Where the conditions are applied

**Decision**: On each bot's own UDP sockets, in the engine's network wrapper that every PeerConnection already opens its sockets through (`consentNet` / `consentConn`, `src/execution/consent.go`). One **shaper per bot** is shared by all the bot's sessions: on LiveKit, mediasoup and Janus a bot has a publish session and a receive session; on Jitsi it has one. Downlink is shaped on reads (`ReadFrom`/`ReadFromUDP`), uplink on writes (`WriteTo`). A bot without a condition gets no shaper: its sockets work as today, with one nil check per datagram.

On the read side, the shaper runs **before** the arrival is recorded for transport-wide feedback (015 R6): a packet "arrives" when it leaves the emulated link, so the bot acknowledges it at the time a browser behind the same link would.

**Rationale**: The wrapper sees every datagram of the bot (ICE, DTLS, SRTP, SRTCP), works with any server address (local or remote, 013), needs no rights (the user's choice), and is per bot by construction.

**Alternatives considered**:
- Kernel traffic control (`tc netem`) on the load machine: refused by the user (administrator rights, selection by port).
- Pion's virtual network (`vnet`): it replaces the whole network with an in-memory router, loses the consent checks of 010, and cannot reach a real server.
- An RTP interceptor: it sits above SRTP, misses ICE, DTLS and RTCP, and would not move the arrival time the bot acknowledges.

## R2. Rate cap model

**Decision**: A token bucket at the cap rate in front of a drop-tail queue, as a router port does. Every datagram is counted with 28 bytes of IPv4 and UDP header. The queue holds at most `queue_ms` of data at the cap rate (default **200 ms**, test plan field). A datagram that does not fit is dropped and counted as dropped by the cap. Delivery uses one goroutine and one timer per shaped direction of a bot, which sleeps until the head of the queue is due.

**Rationale**: Servers' estimators react to queueing delay first (delay-based, as GCC does) and to loss second; a cap without a queue would show only loss, which no real link does. 200 ms is a common home-router buffer.

**Alternatives considered**: dropping above the rate with no queue (loss only, rejected above); an unbounded queue (delay grows without limit and no packet is ever lost, unlike a real link).

## R3. Delay and jitter

**Decision**: Each datagram gets `delay_ms` plus a jitter drawn uniformly in ±`jitter_ms`. Delivery order is kept: a datagram's delivery time is at least the previous one's, so jitter never reorders packets. One jitter value is held for 20 ms (found at T032): drawn per packet, the packets of a video frame waited behind each other, and the mean added delay was 46.5 ms for 40 ± 10. Delay applies after the rate queue.

**Rationale**: Reordering from jitter is a netem artefact more than a property of real access links, and it would make NACK results depend on the shaper. It can be added later as its own field if needed.

## R4. Loss

**Decision**: Random loss is a Bernoulli draw per datagram with probability `share`. Burst loss uses the two-state Gilbert-Elliott model, all packets lost in the bad state: with a mean burst length L and an overall share p, the chance to leave the bad state is 1/L, and to enter it p / (L·(1−p)). `mean_burst` 1 (default) is random loss. Loss applies to every datagram of the direction, ICE and DTLS included, as on a real link.

**Seed**: each bot and direction has its own generator (`math/rand/v2` PCG) seeded from the run's seed, the bot's launch index and the direction. Without a seed in the test plan, the engine draws one and the report gives it (FR-012). The draws do not depend on timing: the same seed gives the same loss pattern per bot over its datagrams (SC-005 counts shares, not exact packets, because the number of datagrams differs slightly between runs).

## R5. Groups, schedule and time

**Decision**:
- A group gives its bots by `count` or `share` of the participants. Bots are assigned from the **end** of the launch order, the first group first: `requested_heights` assigns from the start, so an operator can put the constrained bots in another height group, and the report lists each bot's group and requested height.
- A step has `start_seconds`, an optional `duration_seconds` (none: until the end of the run), and a `downlink` and/or `uplink` condition. Times are counted from the start of the run (the first bot's launch). Steps of a group must not overlap; between steps the group has no condition.
- A step that starts before the last bot's join plus `settle_seconds` (default 10) is flagged in the report (FR-011).
- A bot that joins during a step gets the step in force as soon as its sockets open.

**Rationale**: launch-order assignment is how the test plan already shares bots (006); the end of the order keeps the two independent in the usual plans.

## R6. What the report measures of the condition

**Decision**: The shaper counts, per bot, direction and step: datagrams and bytes offered and delivered, dropped by the cap (queue full) and dropped by loss, and the sum of the delay it added. The report gives per step the delivered rate (mean, and highest over 10-second windows), the dropped shares, and the mean added delay, all measured (FR-006). SC-001's precision is checked by a test that sends through a shaper between two local sockets.

## R7. Sender adaptation (US4)

**Facts**:
- Today no bot's outgoing packets carry transport-wide sequence numbers, so the servers cannot send it transport-wide feedback for its upload. The receive session negotiates the extension only for the feedback the bot sends (015).
- Pion has a send-side estimator (`pkg/gcc`, `SendSideBWE`) fed by transport-wide feedback, with a no-op pacer option. Its increase is capped at 1.5 times the rate the far end received (`rate_controller.go`), and it has no probing. An estimate cannot rise above 1.5 times what is sent, so a bot that stopped a layer will never see an estimate high enough to restore it by waiting.
- The content's keyframes come every 30 frames (1 s, `scripts/gen-media.sh`), in every layer and codec.

**Decision**:
- Only bots of a group whose schedule has an uplink condition adapt. They register Pion's transport-wide sequence number interceptor and the send-side estimator on the session that sends (the publish session, or Jitsi's single session). The estimator starts at the total of the layers sent, so a bot does not start by dropping. Other bots send exactly as today (FR-013, FR-018).
- Layer policy, per sending bot, once per 100 ms:
  - **drop** the top layer still sent after 1 s of continuous congestion, as Pion's estimator reports it: delay-based "overuse", or average loss over 10% (`GetStats`). *Changed at T035 from "the estimate below the total bitrate of the layers sent": after a restore, the estimate rises only 8% per second from where it was, so that rule would drop a restored layer on a link with room for it. The estimate is reported, not used to decide.*
  - **restore** the next layer by trying it: after a wait of 2 s, then 4 s and 8 s at most after each failed try (reset after a success); a restored layer that is dropped again within 3 s counts as a failed try;
  - a layer starts again at its next keyframe (at most 1 s); audio is never dropped.
  - no layer is dropped in the first 5 s after the first feedback (found at T039): Pion's estimator reported congestion in its first second, and both constrained senders dropped a layer 1 s after starting, on a clean path.
  - a layer is not tried while congestion is under way.
- Simulcast (VP8, H.264): the bot stops writing that layer's track. SVC (VP9, AV1 `L3T3_KEY`): the bot stops writing the packets of that spatial layer, read from the dependency descriptor it wrote. In both cases the bot's own video track renumbers what it sends, so the server sees no gap. The shared media source is not changed: dropping happens in the bot's own tracks.
- A single-stream sender does not adapt, and the report says `cannot_adapt` (FR-017).

**Verified (T003, 2026-11-15)** in `pion/interceptor@v0.1.29`:
- `gcc.NewSendSideBWE` takes `SendSideBWEPacer(gcc.NewNoOpPacer())` and `SendSideBWEInitialBitrate`; `OnTargetBitrateChange` exists.
- Sent packets are recorded (`feedbackAdapter.OnSent`) at the time the RTP writer is called, before the bot's socket and so before the shaper: queueing in the shaper shows in the feedback's arrival times.
- `rate_controller.go` `increase` caps the target at 1.5 × the received rate, in both its additive and its multiplicative branch.
- `twcc.NewHeaderExtensionInterceptor` sets an increasing transport-wide sequence number on every packet of a stream that negotiated the extension, and leaves other streams alone. `webrtc.ConfigureTWCCHeaderExtensionSender` registers it with the extension for audio and video. Pion's own example adds the congestion controller to the registry before it.

**Gate result (T004, validation report)**: all four servers send transport-wide feedback to a publisher, every 30–200 ms. LiveKit sends it only when the offer lists the `transport-cc` RTCP feedback as well as the extension, so constrained senders offer both.

**Verified first, per server (gate task)**: that it sends transport-wide feedback to a publisher that offers the extension. That means LiveKit, Janus and Jitsi answering the offered extension, and mediasoup's producer parameters carrying it (`src/platform/mediasoup/sdp.go` maps the SDP extensions). Where a server sends none, its constrained senders cannot adapt; the report says `no_feedback`, and the US4 comparison records it as that server's behaviour.

**Alternatives considered**: padding probes to restore layers (servers handle padding-only packets differently, and Jitsi and LiveKit drop RTP padding, 014); a hand-written estimator (no reason to rewrite GCC); adapting every sender (it would change every existing run and its CPU).

## R8. Reaction and recovery times

**Decision**: From a per-bot timeline the media collector keeps at each collection (every 2 s: `mediaCollectInterval`, found at T016; the resolution is not changed, so as not to change the collector's cost). For each received video stream, it holds the height received and whether a freeze was counted in that interval. For a step that lowers the downlink below what the bot's received streams need:
- the **reaction time** runs from the step's start to the first sample where a stream that was at the requested height is received lower or freezes;
- the **recovery time** runs from the end of the limit to the first sample from which every such stream is at the requested height for 5 s with no freeze, or "not recovered".

Resolution is the collection interval (2 s), stated in the report (`timeline_resolution_seconds`). For uplink steps, the same is computed for the receivers of the constrained sender's streams. The sender of a received stream is the remote participant the adapter names for the track; where none is named, the sender is read from the 012 frame trailer's send-time lookups.

## R9. The server's estimate in the run report

**Decision**: An optional adapter interface (`platform.ReceiverEstimator`): an adapter that can read the server's estimate toward one of its bots through its own signaling implements it. The engine reads it once per second for constrained bots only.
- mediasoup implements it with `getTransportStats` on the receive transport (`availableOutgoingBitrate`, the request of 015's `log_server_stats`).
- Jitsi, LiveKit and Janus give none through their signaling: the report gives the reason `not_given_by_server` (FR-010). For validation, Jitsi's estimate is read by `scripts/read-estimates.py` (015).

**Rationale**: principle III, since what a server's signaling can give is the adapter's business.

## R10. Report shape

**Decision**: Report FormatVersion 4, with a `network` section (contracts/report-network.md):
- the seed, the groups and their steps as applied, with flags;
- per bot: its group, its applied condition per step, its reaction and recovery times, its sender adaptation and the server's estimate;
- one quality summary per group, computed as the run's summary over that group's bots (FR-007).

A run without `network` writes no section and keeps FormatVersion 4 with the field absent.

## R11. CPU

**Decision**: No cost for bots without a condition beyond a nil check per datagram. A shaped direction costs one goroutine, one timer and a queue. A constrained sender adds Pion's sequence number interceptor and estimator. The engine's CPU per bot is measured at 100 bots with no condition (SC-006, before and after), and with 10 bots constrained in both directions; the cost is stated in the validation report.

## R12. Interaction with the 015 findings

- **mediasoup 3.30.1 probe bug** (015 SC-004): mediasoup comparison runs use a 20 s ramp-up for 10 bots, and conditions start after the settling time.
- **Jitsi start-up suspensions** on a LAN: conditions start after `settle_seconds`, and a warm-up run is set aside (013).
- The reference browser (015) is never shaped (spec).

## R13. Entry points

- Test plan: `network` (contracts/test-plan-network.md).
- `cmd/jitsi-connect --network <file.json>`, which reads the `network` object from a file.
- Live tests: `<PLATFORM>_NETWORK=<file.json>`, with the same helper for every platform as 015's `<PLATFORM>_BROWSER_*`.

## R14. REMB where the server asks for it (found at T022)

**Facts** (2026-11-15, local LiveKit 1.13.10, defaults):
- LiveKit's subscriber offers list `a=rtcp-fb:<pt> goog-remb` and the `abs-send-time` extension, and no `transport-cc`.
- Browsers answer with receive-side estimation and send REMB. The bots sent none, so LiveKit never learned a bot's download. Under the US1 cap (500 kbit/s toward 3 bots), it kept forwarding 720: 295 freezes, audio score 2.1, against 4.4 for the others.
- LiveKit has `rtc.congestion_control.use_send_side_bwe` (default false) to use transport-wide feedback instead. The user chose to keep LiveKit's defaults and make the bots send REMB.
- Janus showed the same result, for its own reason: the VideoRoom has no downlink estimation and forwards the substream asked for. That is Janus's behaviour, recorded as such.

**Decision**:
- Every receive session registers the `abs-send-time` extension and the `goog-remb` RTCP feedback, as a browser's answer does, so the server sends the extension where it offers it.
- A session whose applied offer negotiates `goog-remb` and not `transport-cc` runs a receive-side estimator. It is fed at the socket read, with the shaped arrival time (R1), like the transport-wide feedback. It sends a REMB once per second, and at once when the estimate falls by 3% or more, naming the SSRCs it receives. The estimator follows libwebrtc's `RemoteBitrateEstimatorAbsSendTime`:
  - packets are grouped by send time (5 ms bursts);
  - a trendline filter (window 20, gain 4) runs over the inter-group delay variation, with libwebrtc's adaptive threshold (start 12.5 ms, k_up 0.0087, k_down 0.039);
  - an AIMD controller sets the estimate: +8% per second while normal, capped at 1.5 × the incoming rate; 0.85 × the incoming rate on overuse; a hold after a decrease.
- The estimate starts at the incoming rate after the first second of media.
- Pion's GCC is not reused: its delay filters are unexported and only take transport-wide feedback.
- Sessions that negotiate transport-cc (Jitsi, mediasoup, Janus) do not run it: as in a browser, transport-cc takes precedence there.

**Risk**: this changes LiveKit runs without conditions, which until now saw no estimate at all. SC-006/SC-010 check that LiveKit's 014 layered results stay within 0.02.

## R15. Retransmission requests (found at T022)

**Fact**: Pion's NACK generator (`pkg/nack`, defaults) asks for every missing packet every 100 ms with no limit. Under heavy loss on a capped link (Jitsi, US1), retransmissions filled the bot's link 14-fold.

**Decision**: the bots limit the requests per lost packet to 10, as libwebrtc does (`kMaxNackRetries`), through `nack.GeneratorMaxNacksPerPacket(10)` in place of `webrtc.ConfigureNack`. No effect on a clean path.
