package execution

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"webrtc-load-engine/src/api/handlers"
	"webrtc-load-engine/src/scenario"
)

// TestServerLoadThroughAPI creates and runs a scenario with
// server_monitoring over HTTP and reads the server and load_machine
// sections from the quality endpoint (007 quickstart step 6). Docker is
// pointed at a missing socket, so the container is reported unavailable
// without depending on the host.
func TestServerLoadThroughAPI(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := scenario.NewService(logger, NewTestEngineStarter(logger, &IntegrationMockPlatformAdapter{t: t}))
	h := handlers.NewScenarioHandler(service, logger, &NoOpMetrics{})
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/scenarios", h.CreateScenario)
	mux.HandleFunc("/api/v1/scenarios/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/_execute"):
			h.ExecuteScenario(w, r)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/quality"):
			h.GetQuality(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	body, _ := json.Marshal(map[string]any{
		"name":     "server load",
		"platform": "jitsi",
		"test_plan": map[string]any{
			"participants": 2, "ramp_up_seconds": 0, "duration_seconds": 2,
			"server_monitoring": map[string]any{"containers": []string{"docker-jitsi-meet-jvb-1"}, "interval_seconds": 1},
		},
	})
	resp, err := http.Post(ts.URL+"/api/v1/scenarios", "application/json", bytes.NewReader(body))
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %v %v", err, resp.Status)
	}
	var created handlers.ScenarioResponse
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	resp, err = http.Post(ts.URL+"/api/v1/scenarios/"+created.ID+"/_execute", "application/json", nil)
	if err != nil || resp.StatusCode != http.StatusAccepted {
		t.Fatalf("execute: %v %v", err, resp.Status)
	}
	resp.Body.Close()

	var summary map[string]any
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(ts.URL + "/api/v1/scenarios/" + created.ID + "/quality")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode == http.StatusOK {
			json.NewDecoder(resp.Body).Decode(&summary)
			resp.Body.Close()
			break
		}
		resp.Body.Close()
		time.Sleep(200 * time.Millisecond)
	}
	if summary == nil {
		t.Fatal("no quality summary")
	}
	server := summary["server"].(map[string]any)
	if server["available"] != false || server["reason"] != scenario.ReasonDockerUnreachable {
		t.Errorf("server %v, want unavailable, docker_unreachable", server)
	}
	containers := server["containers"].([]any)
	if c := containers[0].(map[string]any); c["name"] != "docker-jitsi-meet-jvb-1" || c["status"] != scenario.LoadStatusUnavailable {
		t.Errorf("container %v", c)
	}
	engine := summary["load_machine"].(map[string]any)["engine"].(map[string]any)
	if engine["samples"].(float64) < 2 {
		t.Errorf("engine %v, want samples", engine)
	}
}
