package hostload

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"webrtc-load-engine/src/scenario"
)

const defaultDockerSocket = "/var/run/docker.sock"

// errNoFigures is a stats answer without figures: the container exists but
// is not running.
var errNoFigures = errors.New("stats without figures")

// dockerClient reads container statistics from the local Docker Engine API
// over its Unix socket (research R1). No Docker SDK: two figures are needed.
type dockerClient struct {
	socket  string
	remote  bool // DOCKER_HOST is not a unix:// URL: out of scope
	timeout time.Duration
	http    *http.Client
}

// newDockerClient returns a client for dockerHost (the DOCKER_HOST value;
// empty for the default socket). Each request gives up after timeout.
func newDockerClient(dockerHost string, timeout time.Duration) *dockerClient {
	c := &dockerClient{socket: defaultDockerSocket, timeout: timeout}
	if dockerHost != "" {
		if path, ok := strings.CutPrefix(dockerHost, "unix://"); ok {
			c.socket = path
		} else {
			c.remote = true
		}
	}
	socket := c.socket
	c.http = &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
		MaxIdleConns:    32,
		IdleConnTimeout: time.Minute,
	}}
	return c
}

// stats returns a container's cumulative CPU time and its memory as
// docker stats shows it, or why they could not be read (a scenario.Reason*).
func (c *dockerClient) stats(ctx context.Context, name string) (cpuNS uint64, memBytes int64, reason string) {
	if c.remote {
		return 0, 0, scenario.ReasonRemoteDockerHost
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://docker/containers/"+url.PathEscape(name)+"/stats?stream=false&one-shot=true", nil)
	if err != nil {
		return 0, 0, scenario.ReasonDockerUnreachable
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, syscall.EACCES) {
			return 0, 0, scenario.ReasonPermissionDenied
		}
		return 0, 0, scenario.ReasonDockerUnreachable
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return 0, 0, scenario.ReasonNotFound
	case resp.StatusCode != http.StatusOK || err != nil:
		return 0, 0, scenario.ReasonDockerUnreachable
	}
	cpuNS, memBytes, err = parseStats(body)
	switch {
	case errors.Is(err, errNoFigures):
		return 0, 0, scenario.ReasonNotRunning
	case err != nil:
		return 0, 0, scenario.ReasonDockerUnreachable
	}
	return cpuNS, memBytes, ""
}

// statsBody is the part of the Engine API's stats answer that is used.
type statsBody struct {
	Read     time.Time `json:"read"`
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
	} `json:"cpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
}

// parseStats returns the cumulative CPU time and the memory without the
// inactive file cache: inactive_file on cgroup v2, total_inactive_file on
// cgroup v1, as the Docker CLI computes it (research R4).
func parseStats(body []byte) (cpuNS uint64, memBytes int64, err error) {
	var s statsBody
	if err := json.Unmarshal(body, &s); err != nil {
		return 0, 0, fmt.Errorf("stats: %w", err)
	}
	if s.Read.IsZero() || s.Read.Year() <= 1 {
		return 0, 0, errNoFigures
	}
	mem := s.MemoryStats.Usage
	for _, key := range []string{"inactive_file", "total_inactive_file"} {
		if v, ok := s.MemoryStats.Stats[key]; ok {
			if v < mem {
				mem -= v
			}
			break
		}
	}
	return s.CPUStats.CPUUsage.TotalUsage, int64(mem), nil
}
