package remote

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const platformsDir = "../../tests/platforms"

func TestLookupServer(t *testing.T) {
	for name, want := range map[string]struct {
		folder     string
		services   []string
		containers []string
		signaling  int
		memoryGB   int
		published  bool
	}{
		"jitsi":     {"docker-jitsi-meet", []string{"web", "prosody", "jicofo", "jvb"}, []string{"sfu-jitsi-jvb-1", "sfu-jitsi-jicofo-1"}, 8000, 10, true},
		"livekit":   {"livekit", []string{"livekit"}, []string{"livekit-server"}, 7880, 2, false},
		"mediasoup": {"mediasoup", []string{"mediasoup"}, []string{"mediasoup-demo"}, 4443, 2, false},
		"janus":     {"janus", []string{"janus"}, []string{"janus"}, 8188, 2, false},
	} {
		s, ok := LookupServer(name)
		if !ok {
			t.Fatalf("%s not found", name)
		}
		if s.Name != name || s.Folder != want.folder || s.MemoryGB != want.memoryGB || s.Published != want.published {
			t.Errorf("%s: %+v", name, s)
		}
		if !slices.Equal(s.Services, want.services) {
			t.Errorf("%s services %v", name, s.Services)
		}
		if !slices.Equal(s.Containers, want.containers) {
			t.Errorf("%s containers %v", name, s.Containers)
		}
		if s.TCPPorts[0] != want.signaling {
			t.Errorf("%s signaling %d", name, s.TCPPorts[0])
		}
		for _, c := range s.Containers {
			if s.Images[c] == "" {
				t.Errorf("%s: no pin for %s", name, c)
			}
		}
	}
	if _, ok := LookupServer("nope"); ok {
		t.Error("unknown server found")
	}
}

func TestServers_Ports(t *testing.T) {
	s, _ := LookupServer("livekit")
	if !slices.Equal(s.UDPPorts, []PortRange{{50000, 60000}}) {
		t.Errorf("livekit %v", s.UDPPorts)
	}
	s, _ = LookupServer("jitsi")
	if !slices.Equal(s.UDPPorts, []PortRange{{10000, 10000}}) {
		t.Errorf("jitsi %v", s.UDPPorts)
	}
	s, _ = LookupServer("janus")
	if !slices.Equal(s.UDPPorts, []PortRange{{20000, 20999}}) {
		t.Errorf("janus %v", s.UDPPorts)
	}
	s, _ = LookupServer("mediasoup")
	if !slices.Equal(s.TCPPorts, []int{4443, 44444, 44445}) || !slices.Equal(s.UDPPorts, []PortRange{{44444, 44445}}) {
		t.Errorf("mediasoup %v %v", s.TCPPorts, s.UDPPorts)
	}
}

// composeService is what the tests read from a compose file.
type composeService struct{ image, containerName string }

var (
	serviceRE = regexp.MustCompile(`^( +)([a-z][a-z-]*):\s*$`)
	imageRE   = regexp.MustCompile(`^\s+image:\s*(\S+)`)
	nameRE    = regexp.MustCompile(`^\s+container_name:\s*(\S+)`)
)

// composeServices reads the services of a compose file: names at the first
// indent under "services:". No YAML module, as the files are simple.
func composeServices(t *testing.T, path string) map[string]*composeService {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*composeService{}
	var cur *composeService
	indent := ""
	in := false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "services:") {
			in = true
			continue
		}
		if !in {
			continue
		}
		if m := serviceRE.FindStringSubmatch(line); m != nil && (indent == "" || m[1] == indent) {
			indent = m[1]
			cur = &composeService{}
			out[m[2]] = cur
			continue
		}
		if cur == nil {
			continue
		}
		if m := imageRE.FindStringSubmatch(line); m != nil {
			cur.image = m[1]
		}
		if m := nameRE.FindStringSubmatch(line); m != nil {
			cur.containerName = m[1]
		}
	}
	if len(out) == 0 {
		t.Fatalf("no services in %s", path)
	}
	return out
}

// The pins are the compose files' versions: a version bump there that is not
// made here must fail (FR-003).
func TestServers_PinsMatchComposeFiles(t *testing.T) {
	for _, name := range []string{"livekit", "mediasoup", "janus"} {
		s, _ := LookupServer(name)
		services := composeServices(t, filepath.Join(platformsDir, s.Folder, "docker-compose.yml"))
		for _, svc := range s.Services {
			c := services[svc]
			if c == nil {
				t.Fatalf("%s: no service %s", name, svc)
			}
			if got := s.Images[c.containerName]; got != c.image {
				t.Errorf("%s/%s pin %q, compose file %q", name, svc, got, c.image)
			}
		}
	}
	s, _ := LookupServer("jitsi")
	services := composeServices(t, filepath.Join(platformsDir, s.Folder, "docker-compose.yml"))
	for _, svc := range s.Services {
		c := services[svc]
		if c == nil {
			t.Fatalf("jitsi: no service %s", svc)
		}
		want := "${JITSI_IMAGE_REPO:-ghcr.io/jitsi}/" + svc + ":${JITSI_IMAGE_VERSION:-unstable}"
		if c.image != want {
			t.Errorf("jitsi/%s image %q", svc, c.image)
		}
		if got := s.Images["sfu-jitsi-"+svc+"-1"]; got != "ghcr.io/jitsi/"+svc+":"+jitsiVersion {
			t.Errorf("jitsi/%s pin %q", svc, got)
		}
	}
}

func TestServers_MissingFolderIsAnError(t *testing.T) {
	for _, name := range []string{"jitsi", "livekit", "mediasoup", "janus"} {
		s, _ := LookupServer(name)
		if _, err := os.Stat(filepath.Join(platformsDir, s.Folder, "env.example")); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A reboot must not bring a server back without its firewall rules (R4).
func TestOverlays_NoRestart(t *testing.T) {
	for _, name := range []string{"jitsi", "livekit", "mediasoup", "janus"} {
		s, _ := LookupServer(name)
		base := composeServices(t, filepath.Join(platformsDir, s.Folder, "docker-compose.yml"))
		b, err := os.ReadFile(filepath.Join(platformsDir, s.Folder, overlay))
		if err != nil {
			t.Fatal(err)
		}
		overlayServices := composeServices(t, filepath.Join(platformsDir, s.Folder, overlay))
		if got, want := strings.Count(string(b), `restart: "no"`), len(base); got != want {
			t.Errorf("%s: %d restart lines, %d services", name, got, want)
		}
		for svc := range base {
			if overlayServices[svc] == nil {
				t.Errorf("%s: %s has no overlay", name, svc)
			}
		}
	}
}
