package remote

// jitsiVersion is the stack validated with this project. The local compose
// file defaults to unstable, so a deployment pins it (research R2).
const jitsiVersion = "stable-11248"

// PortRange is an inclusive range of ports.
type PortRange struct {
	First, Last int
}

// Server is one of the setups in tests/platforms (data-model.md).
type Server struct {
	Name   string
	Folder string // under tests/platforms
	// Services are the Compose services started.
	Services []string
	// Containers are the ones server monitoring names.
	Containers []string
	// TCPPorts has the signaling port first.
	TCPPorts []int
	UDPPorts []PortRange
	MemoryGB int
	// Images gives the pinned image of every container started.
	Images map[string]string
	// Published is true when ports go through Docker rather than host
	// networking, so that firewall rules belong in DOCKER-USER.
	Published bool
	// SecretKeys are the .env keys generated for each deployment.
	SecretKeys []string
	// Ready, when set, is a command on the machine that succeeds once a bot
	// can join, which an open port does not always mean; Waiting names it.
	Ready, Waiting string
}

var servers = []Server{
	{
		Name:       "jitsi",
		Folder:     "docker-jitsi-meet",
		Services:   []string{"web", "prosody", "jicofo", "jvb"},
		Containers: []string{"sfu-jitsi-jvb-1", "sfu-jitsi-jicofo-1"},
		TCPPorts:   []int{8000, 8443},
		UDPPorts:   []PortRange{{10000, 10000}},
		MemoryGB:   10,
		Images: map[string]string{
			"sfu-jitsi-web-1":     "ghcr.io/jitsi/web:" + jitsiVersion,
			"sfu-jitsi-prosody-1": "ghcr.io/jitsi/prosody:" + jitsiVersion,
			"sfu-jitsi-jicofo-1":  "ghcr.io/jitsi/jicofo:" + jitsiVersion,
			"sfu-jitsi-jvb-1":     "ghcr.io/jitsi/jvb:" + jitsiVersion,
		},
		Published:  true,
		SecretKeys: []string{"JICOFO_AUTH_PASSWORD", "JVB_AUTH_PASSWORD"},
		// Its ports open before jicofo has a bridge; a bot joining then gets
		// "conference focus unavailable".
		Ready:   `docker exec sfu-jitsi-jicofo-1 curl -sf -m 2 http://127.0.0.1:8888/debug | grep -q '"operational":true'`,
		Waiting: "jicofo has no operational bridge",
	},
	{
		Name:       "livekit",
		Folder:     "livekit",
		Services:   []string{"livekit"},
		Containers: []string{"livekit-server"},
		TCPPorts:   []int{7880, 7881},
		UDPPorts:   []PortRange{{50000, 60000}},
		MemoryGB:   2,
		Images:     map[string]string{"livekit-server": "livekit/livekit-server:v1.13.10"},
		SecretKeys: []string{"LIVEKIT_API_SECRET"},
	},
	{
		Name:       "mediasoup",
		Folder:     "mediasoup",
		Services:   []string{"mediasoup"},
		Containers: []string{"mediasoup-demo"},
		TCPPorts:   []int{4443, 44444, 44445},
		UDPPorts:   []PortRange{{44444, 44445}},
		MemoryGB:   2,
		Images:     map[string]string{"mediasoup-demo": "webrtc-load-engine/mediasoup-demo:899109e"},
	},
	{
		Name:       "janus",
		Folder:     "janus",
		Services:   []string{"janus"}, // not janus-demos: only the media server is measured
		Containers: []string{"janus"},
		TCPPorts:   []int{8188},
		UDPPorts:   []PortRange{{20000, 20999}},
		MemoryGB:   2,
		Images:     map[string]string{"janus": "webrtc-load-engine/janus:v1.4.3"},
	},
}

// LookupServer returns the named server.
func LookupServer(name string) (Server, bool) {
	for _, s := range servers {
		if s.Name == name {
			return s, true
		}
	}
	return Server{}, false
}

// project is the Compose project name, which bounds what remove touches.
func (s Server) project() string { return "sfu-" + s.Name }
