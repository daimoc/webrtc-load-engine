package media

import (
	"strings"
	"testing"
)

func TestManifest_VP8Entries(t *testing.T) {
	m, err := loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	want := map[int]int{90: 60, 180: 150, 360: 500, 720: 1500}
	for h, kbps := range want {
		e, err := m.lookup(CodecVP8, ArrangementSingle, h)
		if err != nil {
			t.Fatalf("VP8 %d: %v", h, err)
		}
		if e.BitrateKbps != kbps {
			t.Errorf("VP8 %d: %d kbit/s, want %d", h, e.BitrateKbps, kbps)
		}
		if e.VMAF != e.VMAFReference {
			t.Errorf("VP8 %d: vmaf %g differs from its own reference %g", h, e.VMAF, e.VMAFReference)
		}
	}
}

func TestContentBitrates(t *testing.T) {
	if got := ContentBitrates(CodecVP8, ArrangementSingle, 360); len(got) != 1 || got[360] != 500 {
		t.Fatalf("single 360: %v", got)
	}
	got := ContentBitrates(CodecVP8, ArrangementSimulcast, 720)
	if len(got) != 3 || got[180] != 150 || got[360] != 500 || got[720] != 1500 {
		t.Fatalf("simulcast 720: %v", got)
	}
}

func TestManifest_Missing(t *testing.T) {
	m := manifest{{Codec: CodecVP8, Arrangement: ArrangementSingle, Height: 360, BitrateKbps: 500}}
	_, err := m.lookup(CodecAV1, ArrangementSVC, 720)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, s := range []string{"av1", "svc", "720"} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("error %q does not name %s", err, s)
		}
	}
}
