package scenario

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"
)

// jsonKeys returns the dotted paths of the keys of a JSON object.
func jsonKeys(prefix string, v any, out map[string]bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	for k, child := range m {
		out[prefix+k] = true
		jsonKeys(prefix+k+".", child, out)
	}
}

func TestQualitySummary_JSON(t *testing.T) {
	p := &Percentiles{P50: 1, P95: 2, P99: 3}
	s := QualitySummary{
		ScenarioID:  "s",
		GeneratedAt: time.Unix(0, 0).UTC(),
		Bots:        2,
		Video: VideoQuality{
			KindQuality:   KindQuality{Available: true, Bots: 2, Latency: p, LatencyAvailable: true},
			FrameRate:     p,
			FreezeSeconds: p,
			HeightShare:   map[string]float64{"360": 1},
		},
		Audio: AudioQuality{
			KindQuality:    KindQuality{Available: false, Reason: "no bot received audio"},
			ConcealedRatio: p,
			MOS:            p,
		},
		Worst: WorstBots{LowestMOS: []BotValue{{PeerID: "a", Value: 4}}},
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	jsonKeys("", decoded, got)
	for _, k := range strings.Fields(`scenario_id generated_at bots
		video.available video.bots video.latency_seconds.p50 video.latency_seconds.p95 video.latency_seconds.p99
		video.latency_available video.frame_rate.p50 video.freeze_seconds.p99 video.freezes_total
		video.resolution_switches_total video.keyframe_requests_sent_total video.keyframe_requests_received_total
		video.height_share.360
		audio.available audio.reason audio.concealed_ratio.p50 audio.mos.p95 audio.latency_available
		worst.lowest_mos worst.most_freeze_seconds worst.highest_latency_p95_seconds`) {
		if !got[k] {
			keys := make([]string, 0, len(got))
			for g := range got {
				keys = append(keys, g)
			}
			sort.Strings(keys)
			t.Fatalf("missing key %q in %v", k, keys)
		}
	}
}
