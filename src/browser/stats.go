package browser

import (
	"slices"
	"strconv"

	"webrtc-load-engine/src/scenario"
)

// injectedScript runs in every document of the page before the page's own
// scripts (research R2). It notes every RTCPeerConnection the page uses by
// patching the native prototype's methods, not the constructor: a page
// library may replace the constructor with one around the native one
// (the mediasoup demo does, R12 fact 2), and every connection still calls
// these methods. __wleCollect returns, for every open connection, its
// inbound video streams as a JSON string.
const injectedScript = `(() => {
  const P = window.RTCPeerConnection && window.RTCPeerConnection.prototype;
  if (!P || window.__wleConns) return;
  const conns = window.__wleConns = new Set();
  for (const m of ['setLocalDescription', 'setRemoteDescription', 'addTransceiver', 'addTrack', 'createDataChannel', 'createOffer', 'createAnswer']) {
    const f = P[m];
    if (typeof f !== 'function') continue;
    P[m] = function (...a) { conns.add(this); return f.apply(this, a); };
  }
  window.__wleCollect = async () => {
    const streams = [];
    for (const pc of conns) {
      if (pc.connectionState === 'closed') { conns.delete(pc); continue; }
      const stats = await pc.getStats();
      stats.forEach(r => {
        if (r.type === 'inbound-rtp' && r.kind === 'video') {
          streams.push({ id: r.id, h: r.frameHeight || 0, fd: r.framesDecoded || 0, fc: r.freezeCount || 0,
            tfd: r.totalFreezesDuration || 0, b: r.bytesReceived || 0 });
        }
      });
    }
    return JSON.stringify({ streams, connections: conns.size });
  };
})();`

// collectExpression is evaluated once per sample.
const collectExpression = `window.__wleCollect ? window.__wleCollect() : JSON.stringify({streams: [], injected: false, href: location.href})`

// streamStat is one inbound video stream of a sample, as getStats gives it.
type streamStat struct {
	ID              string  `json:"id"`
	Height          int     `json:"h"`
	FramesDecoded   int     `json:"fd"`
	FreezeCount     int64   `json:"fc"`
	FreezesDuration float64 `json:"tfd"`
	BytesReceived   float64 `json:"b"`
}

// streamState is what the accumulator keeps of a stream between samples.
type streamState struct {
	framesDecoded   int
	bytes           float64
	freezes         int64
	freezesDuration float64
}

// accumulator turns per-second samples into the browser's result (research
// R2). A stream counts in a sample when it decoded at least one frame since
// the previous one; streams that never decode (mediasoup's probation
// stream) are left out.
type accumulator struct {
	streams   map[string]*streamState
	samples   []scenario.BrowserSample
	lastT     float64
	seconds   map[int]float64 // Height → decoded stream-seconds
	total     float64         // All decoded stream-seconds
	bytes     float64         // Video bytes received
	maxActive int
	decoded   bool
}

// add records the sample taken t seconds after the page's load.
func (a *accumulator) add(t float64, stats []streamStat) {
	if a.streams == nil {
		a.streams = map[string]*streamState{}
		a.seconds = map[int]float64{}
	}
	dt := t - a.lastT
	if dt <= 0 {
		return
	}
	a.lastT = t
	var heights []int
	var bytes float64
	for _, s := range stats {
		prev := a.streams[s.ID]
		if prev == nil {
			prev = &streamState{}
			a.streams[s.ID] = prev
		}
		if d := s.BytesReceived - prev.bytes; d > 0 {
			bytes += d
		}
		if s.FramesDecoded > prev.framesDecoded && s.Height > 0 {
			heights = append(heights, s.Height)
			a.seconds[s.Height] += dt
			a.total += dt
		}
		prev.framesDecoded, prev.bytes = s.FramesDecoded, s.BytesReceived
		prev.freezes, prev.freezesDuration = s.FreezeCount, s.FreezesDuration
	}
	slices.Sort(heights)
	if heights == nil {
		heights = []int{}
	}
	if len(heights) > 0 {
		a.decoded = true
	}
	a.maxActive = max(a.maxActive, len(heights))
	a.bytes += bytes
	a.samples = append(a.samples, scenario.BrowserSample{T: t, Heights: heights, Kbps: bytes * 8 / 1000 / dt})
}

// decodedAny reports whether any stream decoded a frame.
func (a *accumulator) decodedAny() bool { return a.decoded }

// summarize fills r's measures; expected is the plan's expected height, 0
// when not stated.
func (a *accumulator) summarize(r *scenario.BrowserResult, expected int) {
	r.FreezeSource = "browser"
	r.VideoStreams = a.maxActive
	r.Samples = a.samples
	r.HeightShare = map[string]float64{}
	for h, s := range a.seconds {
		r.HeightShare[strconv.Itoa(h)] = s / a.total
	}
	for _, s := range a.streams {
		r.FreezesTotal += s.freezes
		r.FreezeSeconds += s.freezesDuration
	}
	if a.lastT > 0 {
		r.ReceivedBitrateKbps.Average = a.bytes * 8 / 1000 / a.lastT
	}
	for _, s := range a.samples {
		r.ReceivedBitrateKbps.Peak = max(r.ReceivedBitrateKbps.Peak, s.Kbps)
	}
	r.ExpectedHeight = expected
	if expected == 0 {
		r.ExpectedHeightReason = scenario.ExpectedHeightNotStated
		return
	}
	share := 0.0
	if a.total > 0 {
		share = a.seconds[expected] / a.total
	}
	r.ExpectedHeightShare = &share
	for _, s := range a.samples {
		if slices.Contains(s.Heights, expected) {
			t := s.T
			r.SecondsToExpectedHeight = &t
			break
		}
	}
}
