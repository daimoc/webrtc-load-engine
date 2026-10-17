package hostload

import (
	"os"
	"testing"
)

func TestParseStats(t *testing.T) {
	cases := []struct {
		file   string
		cpuNS  uint64
		memory int64
	}{
		// Recorded from the local bridge: usage − inactive_file, as docker stats shows (3.456 GiB).
		{"testdata/stats_cgroup_v2.json", 2348874564000, 3711164416 - 57344},
		{"testdata/stats_cgroup_v1.json", 5000000000, 104857600 - 10485760},
	}
	for _, tc := range cases {
		body, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		cpu, mem, err := parseStats(body)
		if err != nil {
			t.Fatalf("%s: %v", tc.file, err)
		}
		if cpu != tc.cpuNS || mem != tc.memory {
			t.Errorf("%s: cpu %d mem %d, want %d %d", tc.file, cpu, mem, tc.cpuNS, tc.memory)
		}
	}

	cpu, mem, err := parseStats([]byte(`{"read":"2026-10-16T21:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":7}},"memory_stats":{"usage":99}}`))
	if err != nil || cpu != 7 || mem != 99 {
		t.Errorf("no cache figure: %d %d %v, want usage as is", cpu, mem, err)
	}
	if _, _, err := parseStats([]byte(`{`)); err == nil {
		t.Error("truncated body accepted")
	}
	// A stopped container answers 200 with a zero read time and empty figures.
	if _, _, err := parseStats([]byte(`{"read":"0001-01-01T00:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":0}},"memory_stats":{}}`)); err != errNoFigures {
		t.Errorf("empty figures: %v, want errNoFigures", err)
	}
}
