package janus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// golden decodes a message recorded from Janus 1.4.3 (T002).
func golden(t *testing.T, name string) *Message {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "golden", name))
	if err != nil {
		t.Fatal(err)
	}
	var m Message
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return &m
}

func TestGolden_AllDecode(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "golden", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no goldens: %v", err)
	}
	for _, f := range files {
		golden(t, filepath.Base(f))
	}
}

func TestGolden_Core(t *testing.T) {
	if m := golden(t, "create_session.json"); m.Janus != "success" || m.Data == nil || m.Data.ID == 0 || m.Transaction == "" {
		t.Errorf("create = %+v", m)
	}
	if m := golden(t, "attach.json"); m.Janus != "success" || m.Data == nil || m.Data.ID == 0 {
		t.Errorf("attach = %+v", m)
	}
	if m := golden(t, "ack.json"); m.Janus != "ack" || m.Transaction == "" {
		t.Errorf("ack = %+v", m)
	}
	if m := golden(t, "error_core.json"); m.Janus != "error" || m.Error == nil || m.Error.Code != 458 || m.Error.Reason == "" {
		t.Errorf("core error = %+v", m)
	}
	if m := golden(t, "timeout.json"); m.Janus != "timeout" || m.SessionID == 0 {
		t.Errorf("timeout = %+v", m)
	}
	if m := golden(t, "hangup.json"); m.Janus != "hangup" || m.Sender == 0 || m.Reason != "Close PC" {
		t.Errorf("hangup = %+v", m)
	}
	if m := golden(t, "webrtcup.json"); m.Janus != "webrtcup" || m.Sender == 0 {
		t.Errorf("webrtcup = %+v", m)
	}
	if m := golden(t, "media.json"); m.Janus != "media" || m.MID != "0" || m.Type != "audio" || m.Receiving == nil || !*m.Receiving {
		t.Errorf("media = %+v", m)
	}
}

func TestGolden_Room(t *testing.T) {
	if e := golden(t, "room_created.json").Event(); e == nil || e.VideoRoom != "created" || e.Room != 9002 {
		t.Errorf("created = %+v", e)
	}
	if e := golden(t, "room_exists.json").Event(); e == nil || e.ErrorCode != 427 || e.Error == "" {
		t.Errorf("exists = %+v", e)
	}
	for name, code := range map[string]int{"error_plugin_432.json": 432, "error_plugin_429_pin.json": 429} {
		if e := golden(t, name).Event(); e == nil || e.ErrorCode != code || e.Error == "" {
			t.Errorf("%s = %+v", name, e)
		}
	}
}

func TestGolden_Publisher(t *testing.T) {
	m := golden(t, "joined_publisher.json")
	e := m.Event()
	if m.Sender == 0 || e == nil || e.VideoRoom != "joined" || e.ID == 0 || e.PrivateID == 0 || len(e.Publishers) != 1 {
		t.Fatalf("joined = %+v %+v", m, e)
	}
	// IDs above 2^53 / 2 must survive decoding exactly.
	if d := e.Publishers[0]; !d.Dummy || d.ID != 2646808844340002 || len(d.Streams) != 1 || d.Streams[0].Type != "audio" || d.Streams[0].MID != "0" {
		t.Errorf("dummy = %+v", d)
	}
	e = golden(t, "event_publishers.json").Event()
	if e == nil || len(e.Publishers) != 1 || e.Publishers[0].Display != "probe-b" || len(e.Publishers[0].Streams) != 2 ||
		e.Publishers[0].Streams[1].Type != "video" || e.Publishers[0].Streams[1].MID != "1" || e.Publishers[0].Dummy {
		t.Errorf("publishers = %+v", e)
	}
	m = golden(t, "configured.json")
	if e := m.Event(); e == nil || e.Configured != "ok" || m.JSEP == nil || m.JSEP.Type != "answer" {
		t.Errorf("configured = %+v %+v", m, e)
	}
	for name, want := range map[string]IDOrOK{
		"unpublished.json": {ID: 7342466929253127}, "unpublished_second.json": {ID: 7342466929253127}, "unpublished_ok.json": {OK: true},
	} {
		if e := golden(t, name).Event(); e == nil || e.Unpublished == nil || *e.Unpublished != want {
			t.Errorf("%s = %+v", name, e)
		}
	}
	for name, want := range map[string]IDOrOK{"leaving.json": {ID: 7342466929253127}, "leaving_ok.json": {OK: true}} {
		if e := golden(t, name).Event(); e == nil || e.Leaving == nil || *e.Leaving != want {
			t.Errorf("%s = %+v", name, e)
		}
	}
}

func TestGolden_Subscriber(t *testing.T) {
	m := golden(t, "attached_subscriber.json")
	e := m.Event()
	if e == nil || e.VideoRoom != "attached" || m.JSEP == nil || m.JSEP.Type != "offer" || len(e.Streams) != 1 || e.Streams[0].FeedID != 2646808844340002 {
		t.Errorf("attached = %+v %+v", m, e)
	}
	if e := golden(t, "started.json").Event(); e == nil || e.Started != "ok" {
		t.Errorf("started = %+v", e)
	}
	for _, name := range []string{"updated.json", "updated_unsolicited.json"} {
		m := golden(t, name)
		if e := m.Event(); e == nil || e.VideoRoom != "updated" || m.JSEP == nil || len(e.Streams) < 2 {
			t.Errorf("%s = %+v %+v", name, m, e)
		}
	}
	if m := golden(t, "updated_unsolicited.json"); m.Transaction != "" {
		t.Errorf("unsolicited updated has transaction %q", m.Transaction)
	}
	if e := golden(t, "updating.json").Event(); e == nil || e.VideoRoom != "updating" {
		t.Errorf("updating = %+v", e)
	}
}

func TestIDOrOK_RoundTrip(t *testing.T) {
	for _, v := range []IDOrOK{{ID: 7342466929253127}, {OK: true}} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var got IDOrOK
		if err := json.Unmarshal(b, &got); err != nil || got != v {
			t.Errorf("%s -> %+v, %v", b, got, err)
		}
	}
	var bad IDOrOK
	if err := json.Unmarshal([]byte(`"nope"`), &bad); err == nil {
		t.Error("accepted \"nope\"")
	}
}
