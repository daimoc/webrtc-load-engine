package execution

import (
	"sort"
	"strconv"

	"webrtc-load-engine/src/media"
	"webrtc-load-engine/src/scenario"
)

// codecPeer is what the codec result reads from a peer, whatever its state:
// a bot that failed on a codec mismatch is part of the result.
type codecPeer interface {
	ID() string
	CodecReport() (received []string, mismatch *scenario.CodecMismatch)
}

// codecResult is the codec part of the video summary of an audio_video run
// (contracts/report-codec.md); nil for other runs.
func codecResult(m *scenario.MediaConfig, peers []codecPeer) *scenario.VideoCodecResult {
	if m == nil || m.Profile != scenario.MediaProfileAudioVideo {
		return nil
	}
	maxHeight := 0
	if m.Simulcast != nil {
		maxHeight = m.Simulcast.MaxHeight
	}
	r := &scenario.VideoCodecResult{
		Sent:               string(m.Codec()),
		Arrangement:        string(m.Arrangement()),
		ContentBitrateKbps: map[string]int{},
		Received:           map[string]int{},
		Mismatch:           []scenario.CodecMismatch{},
	}
	for h, k := range media.ContentBitrates(m.Codec(), m.Arrangement(), maxHeight) {
		r.ContentBitrateKbps[strconv.Itoa(h)] = k
	}
	for _, p := range peers {
		received, mismatch := p.CodecReport()
		for _, c := range received {
			r.Received[c]++
		}
		if mismatch != nil {
			r.Mismatch = append(r.Mismatch, *mismatch)
		}
	}
	sort.Slice(r.Mismatch, func(i, j int) bool { return r.Mismatch[i].Bot < r.Mismatch[j].Bot })
	return r
}

// codecPeers returns all the scenario's peers, failed ones included.
func (e *Engine) codecPeers() []codecPeer {
	e.peersMutex.RLock()
	defer e.peersMutex.RUnlock()
	out := make([]codecPeer, 0, len(e.peers))
	for _, p := range e.peers {
		out = append(out, p)
	}
	return out
}

// maxRequestedHeight is the highest video height a bot of the plan asks
// for: the highest requested height, or by default the highest sent.
func maxRequestedHeight(m *scenario.MediaConfig) int {
	if m == nil || len(m.RequestedHeights) == 0 {
		return m.HeightFor(0, 1)
	}
	h := 0
	for _, r := range m.RequestedHeights {
		h = max(h, r.Height)
	}
	return h
}
