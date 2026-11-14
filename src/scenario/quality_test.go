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

// A layered run whose bots received only the lowest layer, though some
// asked for more, sets layers_not_forwarded (014 T057); a single-stream
// run, a run asking only for the lowest layer, and a run where a higher
// layer arrived do not.
func TestVideoQuality_MarkLayersNotForwarded(t *testing.T) {
	layered := map[string]int{"180": 150, "360": 500, "720": 1500}
	cases := []struct {
		name         string
		arrangement  string
		bitrates     map[string]int
		heightShare  map[string]float64
		maxRequested int
		want         bool
	}{
		{"only lowest", "simulcast", layered, map[string]float64{"180": 1}, 720, true},
		{"svc only lowest", "svc", layered, map[string]float64{"180": 1}, 720, true},
		{"higher layer arrived", "simulcast", layered, map[string]float64{"180": 0.5, "720": 0.5}, 720, false},
		{"all asked lowest", "simulcast", layered, map[string]float64{"180": 1}, 180, false},
		{"single stream", "single", map[string]int{"360": 500}, map[string]float64{"360": 1}, 360, false},
		{"no video received", "simulcast", layered, nil, 720, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := VideoQuality{HeightShare: tc.heightShare, Codec: &VideoCodecResult{Arrangement: tc.arrangement, ContentBitrateKbps: tc.bitrates}}
			v.MarkLayersNotForwarded(tc.maxRequested)
			if v.Codec.LayersNotForwarded != tc.want {
				t.Errorf("layers_not_forwarded %v, want %v", v.Codec.LayersNotForwarded, tc.want)
			}
		})
	}
	var none VideoQuality
	none.MarkLayersNotForwarded(720) // No codec result: nothing to mark
}

func marshalKeys(t *testing.T, v any) map[string]bool {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	jsonKeys("", decoded, got)
	return got
}

func TestReceptionReports_JSON(t *testing.T) {
	p := &Percentiles{P50: 1, P95: 1, P99: 0.97}
	r := ReceptionReports{
		Available: true, Bots: 1, AcknowledgedShare: p, LateShare: p, StampLagMaxMs: p,
		Limits:  ReceptionLimits{AcknowledgedShareMin: 0.99, LateShareMax: 0.01, LateIntervalMs: 200},
		Flagged: []string{"bot-7"},
		PerBot: []BotReception{{PeerID: "bot-7", PacketsArrived: 10, PacketsAcknowledged: 9, AcknowledgedShare: 0.9,
			FeedbacksSent: 5, IntervalMs: MeanMax{100, 400}, LateShare: 0.2, StampLagMs: MeanMax{0.5, 30}}},
	}
	got := marshalKeys(t, r)
	for _, k := range strings.Fields(`available bots acknowledged_share.p50 late_share.p95 stamp_lag_max_ms.p99
		limits.acknowledged_share_min limits.late_share_max limits.late_interval_ms flagged per_bot`) {
		if !got[k] {
			t.Errorf("missing %s in %v", k, got)
		}
	}
	if got["reason"] {
		t.Error("reason given with available measures")
	}
	raw, _ := json.Marshal(r)
	var back ReceptionReports
	if err := json.Unmarshal(raw, &back); err != nil || back.PerBot[0].IntervalMs.Max != 400 || back.Flagged[0] != "bot-7" {
		t.Fatalf("round trip: %v %+v", err, back)
	}
	raw, _ = json.Marshal(ReceptionReports{Available: true})
	if !strings.Contains(string(raw), `"flagged":[]`) || !strings.Contains(string(raw), `"per_bot":[]`) {
		t.Errorf("empty lists must be [], got %s", raw)
	}
}

func TestReceptionReports_NotAvailableJSON(t *testing.T) {
	raw, err := json.Marshal(ReceptionReports{Reason: ReceptionNoVideo, Bots: 3, PerBot: []BotReception{{PeerID: "x"}}})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"available":false,"reason":"no_video"}` {
		t.Fatalf("got %s", raw)
	}
}

func TestBrowserResult_JSON(t *testing.T) {
	share, secs := 0.94, 3.0
	b := BrowserResult{
		Status: BrowserMeasured, URL: "https://h/r", Window: "1280x720", BrowserVersion: "Chrome/156",
		VideoStreams: 4, HeightShare: map[string]float64{"720": 0.94, "180": 0.06},
		ExpectedHeight: 720, ExpectedHeightShare: &share, SecondsToExpectedHeight: &secs,
		ReceivedBitrateKbps: MeanPeakKbps{5120, 6300}, FreezeSource: "browser",
		Samples: []BrowserSample{{T: 1, Heights: []int{180}, Kbps: 310}},
	}
	got := marshalKeys(t, b)
	for _, k := range strings.Fields(`status url window browser_version video_streams height_share.720 expected_height
		expected_height_share seconds_to_expected_height received_bitrate_kbps.average received_bitrate_kbps.peak
		freezes_total freeze_seconds freeze_source samples`) {
		if !got[k] {
			t.Errorf("missing %s in %v", k, got)
		}
	}
	if got["reason"] || got["expected_height_reason"] {
		t.Errorf("reasons given with measures: %v", got)
	}
}

func TestBrowserResult_UnavailableJSON(t *testing.T) {
	raw, err := json.Marshal(BrowserResult{Status: BrowserUnavailable, Reason: BrowserNotFound, URL: "https://h/r", Window: "1280x720",
		VideoStreams: 2, Samples: []BrowserSample{{T: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"status":"unavailable","reason":"browser_not_found","url":"https://h/r","window":"1280x720"}`
	if string(raw) != want {
		t.Fatalf("want %s, got %s", want, raw)
	}
}
