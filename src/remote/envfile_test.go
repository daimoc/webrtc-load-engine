package remote

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRenderEnv(t *testing.T) {
	example := []byte("# head\nA=old\n#B=192.168.1.10\n# note B=not a key\nC=keep\n")
	got := string(RenderEnv(example, map[string]string{"A": "new", "B": "1.2.3.4", "Z": "9", "Y": "8"}))
	want := "# head\nA=new\nB=1.2.3.4\n# note B=not a key\nC=keep\nY=8\nZ=9\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if again := string(RenderEnv(example, map[string]string{"A": "new", "B": "1.2.3.4", "Z": "9", "Y": "8"})); again != got {
		t.Error("output differs between two renderings")
	}
}

func TestRenderEnv_ActiveLineWinsOverComment(t *testing.T) {
	got := string(RenderEnv([]byte("#A=1\nA=2\n"), map[string]string{"A": "3"}))
	if got != "#A=1\nA=3\n" {
		t.Errorf("got %q", got)
	}
}

func TestRenderEnv_NoFinalNewline(t *testing.T) {
	got := string(RenderEnv([]byte("A=1"), map[string]string{"B": "2"}))
	if got != "A=1\nB=2\n" {
		t.Errorf("got %q", got)
	}
}

func TestGenerateSecret(t *testing.T) {
	a, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := GenerateSecret()
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(a) || a == b {
		t.Errorf("%q %q", a, b)
	}
}

func TestValues(t *testing.T) {
	secrets := map[string]string{}
	for _, tc := range []struct {
		server, iface string
		want          map[string]string
		absent        []string
	}{
		{"jitsi", "eth0", map[string]string{"JVB_ADVERTISE_IPS": "10.0.0.5", "PUBLIC_URL": "https://10.0.0.5:8443", "JITSI_IMAGE_VERSION": jitsiVersion, "CONFIG": "./config"}, nil},
		{"livekit", "eth0", map[string]string{"LIVEKIT_NODE_IP": "10.0.0.5"}, nil},
		{"mediasoup", "eth0", map[string]string{"MEDIASOUP_ANNOUNCED_ADDRESS": "10.0.0.5", "MEDIASOUP_LISTEN_IP": "0.0.0.0", "HTTP_LISTEN_IP": "0.0.0.0", "MEDIASOUP_DOMAIN": "10.0.0.5"}, nil},
		{"janus", "eth0", map[string]string{"JANUS_ICE_ENFORCE_LIST": "eth0"}, []string{"JANUS_NAT_1_1"}},
		{"janus", "", map[string]string{"JANUS_NAT_1_1": "10.0.0.5"}, []string{"JANUS_ICE_ENFORCE_LIST"}},
	} {
		s, _ := LookupServer(tc.server)
		v := Values(s, "10.0.0.5", secrets, tc.iface)
		for k, want := range tc.want {
			if v[k] != want {
				t.Errorf("%s %s = %q, want %q", tc.server, k, v[k], want)
			}
		}
		for _, k := range tc.absent {
			if _, ok := v[k]; ok {
				t.Errorf("%s has %s", tc.server, k)
			}
		}
	}
}

func TestRealEnvExamples_NoEmptySecret(t *testing.T) {
	secretLine := regexp.MustCompile(`(?m)^[A-Z_]*(PASSWORD|SECRET)=(.*)$`)
	for _, name := range []string{"jitsi", "livekit", "mediasoup", "janus"} {
		s, _ := LookupServer(name)
		example, err := os.ReadFile(filepath.Join(platformsDir, s.Folder, "env.example"))
		if err != nil {
			t.Fatal(err)
		}
		secrets, err := NewSecrets(s)
		if err != nil {
			t.Fatal(err)
		}
		out := string(RenderEnv(example, Values(s, "10.0.0.5", secrets, "eth0")))
		for _, m := range secretLine.FindAllStringSubmatch(out, -1) {
			if strings.TrimSpace(m[2]) == "" {
				t.Errorf("%s: %s", name, m[0])
			}
		}
	}
}

func TestParseEnv(t *testing.T) {
	got := ParseEnv([]byte("# c\nA=1\n\nB=x=y\n#C=3\n"))
	if got["A"] != "1" || got["B"] != "x=y" || len(got) != 2 {
		t.Errorf("%v", got)
	}
}
