package remote

import (
	"crypto/rand"
	"encoding/hex"
	"maps"
	"net"
	"slices"
	"strings"
)

// RenderEnv returns example with each key of values set: on the key's active
// line, else on its commented-out line (#KEY=), else on a new line at the end,
// in key order. Other lines stay as they are.
func RenderEnv(example []byte, values map[string]string) []byte {
	lines := strings.Split(strings.TrimSuffix(string(example), "\n"), "\n")
	var extra []string
	for _, key := range slices.Sorted(maps.Keys(values)) {
		line := key + "=" + values[key]
		i := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, key+"=") })
		if i < 0 {
			i = slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "#"+key+"=") })
		}
		if i < 0 {
			extra = append(extra, line)
			continue
		}
		lines[i] = line
	}
	return []byte(strings.Join(append(lines, extra...), "\n") + "\n")
}

// ParseEnv reads the active KEY=value lines of an env file.
func ParseEnv(b []byte) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok && k != "" {
			out[k] = v
		}
	}
	return out
}

// GenerateSecret returns 32 random hex characters.
func GenerateSecret() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// NewSecrets generates one value per secret key of s.
func NewSecrets(s Server) (map[string]string, error) {
	out := make(map[string]string, len(s.SecretKeys))
	for _, k := range s.SecretKeys {
		v, err := GenerateSecret()
		if err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}

// Values returns what a deployment sets in the server's .env (research R2).
// iface is the interface holding address, empty when it is on none, as behind
// NAT.
func Values(s Server, address string, secrets map[string]string, iface string) map[string]string {
	v := map[string]string{}
	maps.Copy(v, secrets)
	switch s.Name {
	case "jitsi":
		v["JVB_ADVERTISE_IPS"] = address
		// The page tells the browser where to connect; unset, it says
		// localhost, so a browser joins whatever Jitsi runs on its own machine.
		v["PUBLIC_URL"] = "https://" + net.JoinHostPort(address, "8443")
		v["JITSI_IMAGE_VERSION"] = jitsiVersion
		v["CONFIG"] = "./config" // inside the deployment folder, for setup.sh and Compose
	case "livekit":
		v["LIVEKIT_NODE_IP"] = address
	case "mediasoup":
		v["MEDIASOUP_ANNOUNCED_ADDRESS"] = address
		v["MEDIASOUP_LISTEN_IP"] = "0.0.0.0" // it binds 127.0.0.1 by default
		v["HTTP_LISTEN_IP"] = "0.0.0.0"
		v["MEDIASOUP_DOMAIN"] = address
	case "janus":
		if iface != "" {
			v["JANUS_ICE_ENFORCE_LIST"] = iface
		} else {
			v["JANUS_NAT_1_1"] = address
		}
	}
	return v
}
