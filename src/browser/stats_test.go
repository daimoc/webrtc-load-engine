package browser

import (
	"math"
	"reflect"
	"testing"

	"webrtc-load-engine/src/scenario"
)

func st(id string, h, fd int, b float64) streamStat {
	return streamStat{ID: id, Height: h, FramesDecoded: fd, BytesReceived: b}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSummarize_Shares(t *testing.T) {
	var a accumulator
	// Two streams at 180 for 2 s, then one moves to 720 for 2 s.
	a.add(1, []streamStat{st("x", 180, 30, 10000), st("y", 180, 30, 10000)})
	a.add(2, []streamStat{st("x", 180, 60, 20000), st("y", 180, 60, 20000)})
	a.add(3, []streamStat{st("x", 720, 90, 145000), st("y", 180, 90, 30000)})
	a.add(4, []streamStat{st("x", 720, 120, 270000), st("y", 180, 120, 40000)})
	var r scenario.BrowserResult
	a.summarize(&r, 720)
	want := map[string]float64{"180": 0.75, "720": 0.25}
	if len(r.HeightShare) != 2 || !near(r.HeightShare["180"], want["180"]) || !near(r.HeightShare["720"], want["720"]) {
		t.Fatalf("height shares %v, want %v", r.HeightShare, want)
	}
	if r.ExpectedHeightShare == nil || !near(*r.ExpectedHeightShare, 0.25) {
		t.Fatalf("expected share %v", r.ExpectedHeightShare)
	}
	if r.SecondsToExpectedHeight == nil || *r.SecondsToExpectedHeight != 3 {
		t.Fatalf("seconds to expected %v", r.SecondsToExpectedHeight)
	}
	if r.VideoStreams != 2 {
		t.Fatalf("video streams %d", r.VideoStreams)
	}
	if len(r.Samples) != 4 || !reflect.DeepEqual(r.Samples[2].Heights, []int{180, 720}) || r.Samples[2].T != 3 {
		t.Fatalf("samples %+v", r.Samples)
	}
}

func TestSummarize_Bitrate(t *testing.T) {
	var a accumulator
	a.add(1, []streamStat{st("x", 360, 30, 62500)})  // 500 kbit/s in the first second
	a.add(2, []streamStat{st("x", 360, 60, 125000)}) // 500
	a.add(3, []streamStat{st("x", 360, 90, 250000)}) // 1000
	var r scenario.BrowserResult
	a.summarize(&r, 0)
	if !near(r.ReceivedBitrateKbps.Peak, 1000) || !near(r.ReceivedBitrateKbps.Average, 2000.0/3) {
		t.Fatalf("bitrate %+v", r.ReceivedBitrateKbps)
	}
	if !near(r.Samples[2].Kbps, 1000) {
		t.Fatalf("sample kbps %g", r.Samples[2].Kbps)
	}
}

func TestSummarize_NoExpectedHeight(t *testing.T) {
	var a accumulator
	a.add(1, []streamStat{st("x", 360, 30, 1)})
	var r scenario.BrowserResult
	a.summarize(&r, 0)
	if r.ExpectedHeightShare != nil || r.SecondsToExpectedHeight != nil || r.ExpectedHeightReason != scenario.ExpectedHeightNotStated {
		t.Fatalf("got share %v, seconds %v, reason %q", r.ExpectedHeightShare, r.SecondsToExpectedHeight, r.ExpectedHeightReason)
	}
}

func TestSummarize_NeverAtExpectedHeight(t *testing.T) {
	var a accumulator
	a.add(1, []streamStat{st("x", 360, 30, 1)})
	var r scenario.BrowserResult
	a.summarize(&r, 720)
	if r.SecondsToExpectedHeight != nil || r.ExpectedHeightShare == nil || *r.ExpectedHeightShare != 0 {
		t.Fatalf("got share %v, seconds %v", r.ExpectedHeightShare, r.SecondsToExpectedHeight)
	}
}

func TestSummarize_UndecodedStreamsLeftOut(t *testing.T) {
	var a accumulator
	// mediasoup's probation stream: bytes, no frame (R12 fact 1).
	a.add(1, []streamStat{st("probe", 0, 0, 5000), st("x", 360, 30, 1)})
	a.add(2, []streamStat{st("probe", 0, 0, 9000), st("x", 360, 30, 2)}) // x decoded nothing in this second
	var r scenario.BrowserResult
	a.summarize(&r, 0)
	if !reflect.DeepEqual(r.HeightShare, map[string]float64{"360": 1}) || r.VideoStreams != 1 {
		t.Fatalf("shares %v, streams %d", r.HeightShare, r.VideoStreams)
	}
	if len(r.Samples[1].Heights) != 0 {
		t.Fatalf("second 2 heights %v", r.Samples[1].Heights)
	}
	if !a.decodedAny() {
		t.Fatal("decodedAny false")
	}
	var none accumulator
	none.add(1, []streamStat{st("probe", 0, 0, 5000)})
	if none.decodedAny() {
		t.Fatal("decodedAny true with no frame")
	}
}

func TestSummarize_Freezes(t *testing.T) {
	var a accumulator
	a.add(1, []streamStat{{ID: "x", Height: 360, FramesDecoded: 30, FreezeCount: 1, FreezesDuration: 0.4}})
	a.add(2, []streamStat{{ID: "x", Height: 360, FramesDecoded: 60, FreezeCount: 2, FreezesDuration: 0.9}, {ID: "y", Height: 360, FramesDecoded: 30, FreezeCount: 1, FreezesDuration: 0.2}})
	// x disappears (connection closed): its counts are kept.
	a.add(3, []streamStat{{ID: "y", Height: 360, FramesDecoded: 60, FreezeCount: 1, FreezesDuration: 0.2}})
	var r scenario.BrowserResult
	a.summarize(&r, 0)
	if r.FreezesTotal != 3 || !near(r.FreezeSeconds, 1.1) || r.FreezeSource != "browser" {
		t.Fatalf("freezes %d %g %q", r.FreezesTotal, r.FreezeSeconds, r.FreezeSource)
	}
	if !near(r.HeightShare["360"], 1) {
		t.Fatalf("shares %v", r.HeightShare)
	}
}
