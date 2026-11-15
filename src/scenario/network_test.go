package scenario

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// networkPlan returns a valid plan of 10 bots, 60 s, audio_video, with net
// as its network section.
func networkPlan(net *NetworkConfig) TestPlan {
	return TestPlan{Participants: 10, RampUpSeconds: 10, DurationSeconds: 50,
		Media: &MediaConfig{Profile: MediaProfileAudioVideo}, Network: net}
}

func intp(v int) *int { return &v }

func capStep(start int) ScheduleStep {
	return ScheduleStep{StartSeconds: start, Downlink: &LinkCondition{RateKbps: 500}}
}

func TestNetwork_Valid(t *testing.T) {
	n := &NetworkConfig{Groups: []BotGroup{{Name: "slow", Count: 3, Schedule: []ScheduleStep{capStep(20)}}}}
	sc, err := NewScenario("n", "jitsi", networkPlan(n))
	if err != nil {
		t.Fatal(err)
	}
	got := sc.TestPlan.Network
	if got.Settle() != DefaultSettleSeconds {
		t.Errorf("settle_seconds = %d, want %d", got.Settle(), DefaultSettleSeconds)
	}
	if q := got.Groups[0].Schedule[0].Downlink.QueueMs; q != DefaultQueueMs {
		t.Errorf("queue_ms = %d, want default %d", q, DefaultQueueMs)
	}
	if n.Groups[0].Schedule[0].Downlink.QueueMs != 0 {
		t.Error("defaults were written into the caller's plan")
	}
}

func TestNetwork_DefaultMeanBurst(t *testing.T) {
	n := &NetworkConfig{Groups: []BotGroup{{Name: "l", Count: 1, Schedule: []ScheduleStep{{Downlink: &LinkCondition{Loss: &Loss{Share: 0.03}}}}}}}
	sc, err := NewScenario("n", "jitsi", networkPlan(n))
	if err != nil {
		t.Fatal(err)
	}
	if mb := sc.TestPlan.Network.Groups[0].Schedule[0].Downlink.Loss.MeanBurst; mb != 1 {
		t.Errorf("mean_burst = %v, want 1", mb)
	}
}

func TestNetwork_Refused(t *testing.T) {
	group := func(g BotGroup) *NetworkConfig { return &NetworkConfig{Groups: []BotGroup{g}} }
	step := func(s ScheduleStep) *NetworkConfig {
		return group(BotGroup{Name: "g", Count: 1, Schedule: []ScheduleStep{s}})
	}
	down := func(c LinkCondition) *NetworkConfig { return step(ScheduleStep{Downlink: &c}) }
	many := func(n int) *NetworkConfig {
		var gs []BotGroup
		for i := 0; i < n; i++ {
			gs = append(gs, BotGroup{Name: "g" + strings.Repeat("x", i), Share: 0.01, Schedule: []ScheduleStep{capStep(0)}})
		}
		return &NetworkConfig{Groups: gs}
	}
	cases := []struct {
		name  string
		net   *NetworkConfig
		field string
	}{
		{"no group", &NetworkConfig{}, "network.groups"},
		{"17 groups", many(17), "network.groups"},
		{"duplicate name", &NetworkConfig{Groups: []BotGroup{{Name: "a", Count: 1, Schedule: []ScheduleStep{capStep(0)}}, {Name: "a", Count: 1, Schedule: []ScheduleStep{capStep(0)}}}}, "name"},
		{"empty name", group(BotGroup{Count: 1, Schedule: []ScheduleStep{capStep(0)}}), "name"},
		{"bad name", group(BotGroup{Name: "Slow!", Count: 1, Schedule: []ScheduleStep{capStep(0)}}), "name"},
		{"long name", group(BotGroup{Name: strings.Repeat("a", 33), Count: 1, Schedule: []ScheduleStep{capStep(0)}}), "name"},
		{"count and share", group(BotGroup{Name: "g", Count: 1, Share: 0.5, Schedule: []ScheduleStep{capStep(0)}}), "count"},
		{"neither", group(BotGroup{Name: "g", Schedule: []ScheduleStep{capStep(0)}}), "count"},
		{"count too big", group(BotGroup{Name: "g", Count: 11, Schedule: []ScheduleStep{capStep(0)}}), "count"},
		{"share over 1", group(BotGroup{Name: "g", Share: 1.5, Schedule: []ScheduleStep{capStep(0)}}), "share"},
		{"negative share", group(BotGroup{Name: "g", Share: -0.1, Schedule: []ScheduleStep{capStep(0)}}), "share"},
		{"groups over participants", &NetworkConfig{Groups: []BotGroup{{Name: "a", Count: 6, Schedule: []ScheduleStep{capStep(0)}}, {Name: "b", Share: 0.5, Schedule: []ScheduleStep{capStep(0)}}}}, "participants"},
		{"no step", group(BotGroup{Name: "g", Count: 1}), "schedule"},
		{"33 steps", group(BotGroup{Name: "g", Count: 1, Schedule: func() []ScheduleStep {
			var s []ScheduleStep
			for i := 0; i < 33; i++ {
				s = append(s, ScheduleStep{StartSeconds: i, DurationSeconds: intp(1), Downlink: &LinkCondition{RateKbps: 100}})
			}
			return s
		}()}), "schedule"},
		{"overlap", group(BotGroup{Name: "g", Count: 1, Schedule: []ScheduleStep{{StartSeconds: 0, DurationSeconds: intp(20), Downlink: &LinkCondition{RateKbps: 100}}, capStep(10)}}), "start_seconds"},
		{"unordered", group(BotGroup{Name: "g", Count: 1, Schedule: []ScheduleStep{{StartSeconds: 30, DurationSeconds: intp(5), Downlink: &LinkCondition{RateKbps: 100}}, {StartSeconds: 10, DurationSeconds: intp(5), Downlink: &LinkCondition{RateKbps: 100}}}}), "start_seconds"},
		{"open step not last", group(BotGroup{Name: "g", Count: 1, Schedule: []ScheduleStep{capStep(0), capStep(30)}}), "start_seconds"},
		{"negative start", step(ScheduleStep{StartSeconds: -1, Downlink: &LinkCondition{RateKbps: 100}}), "start_seconds"},
		{"start after run", step(ScheduleStep{StartSeconds: 60, Downlink: &LinkCondition{RateKbps: 100}}), "start_seconds"},
		{"end after run", step(ScheduleStep{StartSeconds: 50, DurationSeconds: intp(11), Downlink: &LinkCondition{RateKbps: 100}}), "duration_seconds"},
		{"zero duration", step(ScheduleStep{DurationSeconds: intp(0), Downlink: &LinkCondition{RateKbps: 100}}), "duration_seconds"},
		{"no direction", step(ScheduleStep{}), "downlink"},
		{"empty direction", down(LinkCondition{}), "downlink"},
		{"rate too low", down(LinkCondition{RateKbps: 15}), "rate_kbps"},
		{"rate too high", down(LinkCondition{RateKbps: 1000001}), "rate_kbps"},
		{"negative rate", down(LinkCondition{RateKbps: -5}), "rate_kbps"},
		{"queue without rate", down(LinkCondition{QueueMs: 100, DelayMs: 10}), "queue_ms"},
		{"queue too small", down(LinkCondition{RateKbps: 100, QueueMs: 9}), "queue_ms"},
		{"queue too big", down(LinkCondition{RateKbps: 100, QueueMs: 2001}), "queue_ms"},
		{"delay too big", down(LinkCondition{DelayMs: 5001}), "delay_ms"},
		{"negative delay", down(LinkCondition{DelayMs: -1}), "delay_ms"},
		{"jitter over delay", down(LinkCondition{DelayMs: 10, JitterMs: 11}), "jitter_ms"},
		{"negative jitter", down(LinkCondition{DelayMs: 10, JitterMs: -1}), "jitter_ms"},
		{"loss over 1", down(LinkCondition{Loss: &Loss{Share: 1.1}}), "loss.share"},
		{"negative loss", down(LinkCondition{Loss: &Loss{Share: -0.1}}), "loss.share"},
		{"burst under 1", down(LinkCondition{Loss: &Loss{Share: 0.1, MeanBurst: 0.5}}), "mean_burst"},
		{"settle too big", &NetworkConfig{SettleSeconds: intp(301), Groups: []BotGroup{{Name: "g", Count: 1, Schedule: []ScheduleStep{capStep(0)}}}}, "settle_seconds"},
		{"negative settle", &NetworkConfig{SettleSeconds: intp(-1), Groups: []BotGroup{{Name: "g", Count: 1, Schedule: []ScheduleStep{capStep(0)}}}}, "settle_seconds"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewScenario("n", "jitsi", networkPlan(c.net))
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), c.field) {
				t.Errorf("error %q does not name %q", err, c.field)
			}
		})
	}
}

func TestNetwork_UplinkOnReceiveOnly(t *testing.T) {
	plan := networkPlan(&NetworkConfig{Groups: []BotGroup{{Name: "g", Count: 1, Schedule: []ScheduleStep{{Uplink: &LinkCondition{RateKbps: 100}}}}}})
	plan.Media = &MediaConfig{Profile: MediaProfileReceiveOnly}
	if _, err := NewScenario("n", "jitsi", plan); err == nil || !strings.Contains(err.Error(), "uplink") {
		t.Fatalf("err = %v, want uplink refused", err)
	}
}

func TestNetwork_StrictJSON(t *testing.T) {
	for _, doc := range []string{
		`{"groups":[{"name":"g","count":1,"schedule":[{"downlink":{"rate_kbps":100}}]}],"sede":1}`,
		`{"groups":[{"name":"g","count":1,"cap":2,"schedule":[{"downlink":{"rate_kbps":100}}]}]}`,
		`{"groups":[{"name":"g","count":1,"schedule":[{"start":3,"downlink":{"rate_kbps":100}}]}]}`,
		`{"groups":[{"name":"g","count":1,"schedule":[{"downlink":{"rate":100}}]}]}`,
		`{"groups":[{"name":"g","count":1,"schedule":[{"downlink":{"loss":{"share":0.1,"burst":2}}}]}]}`,
	} {
		var n NetworkConfig
		if err := json.Unmarshal([]byte(doc), &n); err == nil {
			t.Errorf("accepted %s", doc)
		}
	}
	var n NetworkConfig
	doc := `{"seed":42,"settle_seconds":5,"groups":[{"name":"slow","share":0.3,"schedule":[{"start_seconds":20,"duration_seconds":20,"downlink":{"rate_kbps":300,"queue_ms":100,"delay_ms":40,"jitter_ms":10,"loss":{"share":0.03,"mean_burst":2}},"uplink":{"rate_kbps":400}}]}]}`
	if err := json.Unmarshal([]byte(doc), &n); err != nil {
		t.Fatal(err)
	}
	if *n.Seed != 42 || *n.SettleSeconds != 5 || n.Groups[0].Share != 0.3 || *n.Groups[0].Schedule[0].DurationSeconds != 20 ||
		n.Groups[0].Schedule[0].Downlink.Loss.MeanBurst != 2 || n.Groups[0].Schedule[0].Uplink.RateKbps != 400 {
		t.Errorf("decoded %+v", n)
	}
}

func TestNetwork_Assign(t *testing.T) {
	n := &NetworkConfig{Groups: []BotGroup{
		{Name: "slow", Count: 3, Schedule: []ScheduleStep{capStep(0)}},
		{Name: "x", Share: 0.2, Schedule: []ScheduleStep{capStep(0)}},
	}}
	got := n.Assign(10)
	want := []string{"", "", "", "", "", "x", "x", "slow", "slow", "slow"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Assign(10) = %q, want %q", got, want)
		}
	}
	// A share rounds, and gives at least one bot.
	small := &NetworkConfig{Groups: []BotGroup{{Name: "s", Share: 0.01, Schedule: []ScheduleStep{capStep(0)}}}}
	if got := small.Assign(10); got[9] != "s" || got[8] != "" {
		t.Errorf("share 0.01 of 10 = %q, want the last bot", got)
	}
	var none *NetworkConfig
	if got := none.Assign(3); len(got) != 3 || got[0] != "" {
		t.Errorf("nil Assign = %q", got)
	}
}

func TestNetwork_GroupSize(t *testing.T) {
	if got := (BotGroup{Share: 0.25}).Size(10); got != 3 { // round(2.5) = 3
		t.Errorf("share 0.25 of 10 = %d, want 3", got)
	}
	if got := (BotGroup{Count: 4}).Size(10); got != 4 {
		t.Errorf("count 4 = %d", got)
	}
}

func TestNetwork_StepEnd(t *testing.T) {
	if end := (ScheduleStep{StartSeconds: 5, DurationSeconds: intp(10)}).End(60); end != 15 {
		t.Errorf("end = %d, want 15", end)
	}
	if end := (ScheduleStep{StartSeconds: 5}).End(60); end != 60 {
		t.Errorf("open end = %d, want 60", end)
	}
}

func TestReadNetworkFile(t *testing.T) {
	dir := t.TempDir()
	good := dir + "/n.json"
	if err := os.WriteFile(good, []byte(`{"groups":[{"name":"g","count":1,"schedule":[{"downlink":{"rate_kbps":100}}]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := ReadNetworkFile(good)
	if err != nil || n.Groups[0].Schedule[0].Downlink.RateKbps != 100 {
		t.Fatalf("%+v, %v", n, err)
	}
	bad := dir + "/b.json"
	if err := os.WriteFile(bad, []byte(`{"groups":[],"x":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadNetworkFile(bad); err == nil {
		t.Error("unknown field accepted")
	}
	if _, err := ReadNetworkFile(dir + "/none.json"); err == nil {
		t.Error("missing file accepted")
	}
}
