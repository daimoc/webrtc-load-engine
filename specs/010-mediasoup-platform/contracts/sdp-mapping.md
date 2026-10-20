# mediasoup parameters ↔ SDP (research R2, R3, R5)

The adapter writes and reads SDP with `pion/sdp/v3`. Golden tests cover
each direction (R12).

## Receive offer (server → bot), built by the adapter

Session level:

```text
v=0
o=- <session id> <version, +1 per offer> IN IP4 0.0.0.0
s=-
t=0 0
a=ice-lite
a=group:BUNDLE datachannel <mid> <mid> …
a=msid-semantic: WMS *
a=fingerprint:sha-256 <dtlsParameters.fingerprints[sha-256].value>
```

Every section has the transport's ICE and DTLS:

```text
c=IN IP4 127.0.0.1                                (address of the first UDP candidate)
a=ice-ufrag:<iceParameters.usernameFragment>
a=ice-pwd:<iceParameters.password>
a=setup:actpass
a=candidate:<foundation> 1 udp <priority> <address> <port> typ host   (UDP candidates only)
a=end-of-candidates
a=rtcp-mux
```

Section 0, always first:

```text
m=application <port> UDP/DTLS/SCTP webrtc-datachannel
a=mid:datachannel
a=sctp-port:<sctpParameters.port>
a=max-message-size:<sctpParameters.maxMessageSize>
```

One section per accepted consumer, in acceptance order:

```text
m=<kind> <port> UDP/TLS/RTP/SAVPF <pt> [<rtx pt>]
a=mid:<rtpParameters.mid>
a=sendonly                                        (a=inactive once closed, without ssrc lines)
a=rtpmap:<pt> <opus/48000/2 | VP8/90000>
a=fmtp:<pt> <codec parameters, e.g. minptime=10;useinbandfec=1>
a=rtcp-fb:<pt> <rtcpFeedback type [parameter]>    (nack, nack pli, ccm fir, goog-remb, transport-cc)
a=rtpmap:<rtx pt> rtx/90000                       (video with RTX)
a=fmtp:<rtx pt> apt=<pt>
a=extmap:<id> <uri>                               (rtpParameters.headerExtensions)
a=msid:<peerId> <consumerId>
a=ssrc-group:FID <ssrc> <rtx ssrc>                (video with RTX)
a=ssrc:<ssrc> cname:<rtcp.cname>
a=ssrc:<rtx ssrc> cname:<rtcp.cname>
```

The bot's answer (Pion) gives `a=setup:passive` (Pion's answer to an
ICE-lite `actpass` offer) and its fingerprint. They are sent as
`connectWebRtcTransport {dtlsParameters: {role: "server", …}}` after the
first answer only: the role is read from the answer (`passive` → `server`,
`active` → `client`).

## Publish offer (bot → server) → producer `rtpParameters`

Pion's offer has one `sendonly` section per local track. For each one:

| `rtpParameters` | From the offer section |
|---|---|
| `mid` | `a=mid` |
| `codecs[0]` | the first `rtpmap` that is Opus 48000/2 (audio) or VP8 90000 (video): `mimeType` (`audio/opus`, `video/VP8`), `payloadType`, `clockRate`, `channels` (audio), `parameters` from its `fmtp`, `rtcpFeedback` from its `rtcp-fb` |
| `codecs[1]` | the `rtx` whose `apt` is that payload type, if present: `video/rtx`, `parameters:{apt}` |
| `headerExtensions` | the offer's `extmap`s whose URI the router supports for that kind: `{uri, id}` |
| `encodings` | `[{ssrc: <first ssrc of the section>, rtx: {ssrc: <FID second ssrc>}?}]` |
| `rtcp` | `{cname: <ssrc cname>, reducedSize: true}` |

`produce` is sent with `kind` from the section's media.

## Publish answer (server → bot), built by the adapter

The same session and ICE lines as the receive offer, from the *sending*
transport, with `a=setup:passive`. One section per offered section, same
order and mid, `a=recvonly`, carrying the codec, RTX and RTCP feedback the
offer had for the produced codec, and the header extensions kept above,
with the offer's IDs. No SSRC lines.
