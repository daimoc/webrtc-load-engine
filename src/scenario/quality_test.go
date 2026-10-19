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

func TestQualitySummary_ServerLoadJSON(t *testing.T) {
	at := time.Unix(0, 0).UTC()
	figures := LoadFigures{
		CPUThreads:  CPUThreads{Average: 1, Peak: 2, First: 0.1},
		MemoryBytes: MemoryBytes{Peak: 3, First: 1},
		Samples:     4,
		Missed:      1,
	}
	s := QualitySummary{
		Server: ServerLoad{
			Available:       true,
			IntervalSeconds: 5,
			Period:          &Period{Start: at, End: at},
			Containers: []ContainerLoad{
				{Name: "jvb", Status: LoadStatusPartial, Reason: ReasonNotRunning, SharedWith: []string{"other"}, LoadFigures: figures},
			},
			Total: &ServerTotal{ContainersMeasured: 1, LoadFigures: figures},
		},
		LoadMachine: LoadMachine{Period: &Period{Start: at, End: at}, Engine: figures, HardwareThreads: 16, UsableThreads: 8},
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
	for _, k := range strings.Fields(`server.available server.interval_seconds server.period.start server.period.end
		server.containers server.total.containers_measured server.total.cpu_threads.average
		server.total.cpu_threads.peak server.total.cpu_threads.first server.total.memory_bytes.peak
		server.total.memory_bytes.first server.total.samples server.total.missed
		load_machine.period.start load_machine.engine.cpu_threads.average load_machine.engine.memory_bytes.peak
		load_machine.engine.samples load_machine.hardware_threads load_machine.usable_threads`) {
		if !got[k] {
			t.Errorf("missing key %q", k)
		}
	}
	containers := decoded.(map[string]any)["server"].(map[string]any)["containers"].([]any)
	c := map[string]bool{}
	jsonKeys("", containers[0], c)
	for _, k := range strings.Fields(`name status reason shared_with cpu_threads.average memory_bytes.peak samples missed`) {
		if !c[k] {
			t.Errorf("missing container key %q", k)
		}
	}
}

func TestKindQuality_LatencyReasonJSON(t *testing.T) {
	raw, err := json.Marshal(KindQuality{Available: true, Bots: 1, LatencyReason: LatencyFrameNotIdentified})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"latency_reason":"frame_not_identified"`) {
		t.Errorf("JSON %s has no latency_reason", raw)
	}
	raw, _ = json.Marshal(KindQuality{Available: true, LatencyAvailable: true})
	if strings.Contains(string(raw), "latency_reason") {
		t.Errorf("JSON %s has latency_reason without a reason", raw)
	}
}
