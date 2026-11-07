package remote

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// SettingsPath gives the workstation's settings file of a target.
func SettingsPath(o Options) string {
	dir := o.SettingsDir
	if dir == "" {
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = filepath.Join(os.Getenv("HOME"), ".config")
		}
		dir = filepath.Join(base, "webrtc-load-engine", "sfu-remote")
	}
	return filepath.Join(dir, o.Target.Host+".env")
}

// serverURL gives the URL the bots connect to.
func serverURL(s Server, address string) string {
	u := "ws://" + net.JoinHostPort(address, strconv.Itoa(s.TCPPorts[0]))
	if s.Name == "jitsi" {
		u += "/xmpp-websocket"
	}
	return u
}

// tlsNote says whether bots need the setting that skips certificate checks
// (FR-008b).
func tlsNote(s Server, url string) string {
	if !strings.HasPrefix(url, "wss://") {
		return "not needed (ws://)"
	}
	return "bots need " + strings.ToUpper(s.Name) + "_INSECURE_TLS=1 with a self-signed certificate"
}

// settingsVars gives the variables the live tests read (contract:
// settings-file.md). env is the server's rendered .env.
func settingsVars(s Server, t Target, address string, env map[string]string) map[string]string {
	v := map[string]string{"DOCKER_HOST": t.SSHURL()}
	containers := strings.Join(s.Containers, ",")
	url := serverURL(s, address)
	switch s.Name {
	case "jitsi":
		host := net.JoinHostPort(address, strconv.Itoa(s.TCPPorts[0]))
		v["JITSI_ROOM_URL"] = "http://" + host + "/loadtest"
		v["JITSI_XMPP_DOMAIN"] = "meet.jitsi"
		v["JITSI_MUC_DOMAIN"] = "muc.meet.jitsi"
		v["JITSI_WS_URL"] = url
		v["JITSI_SERVER_CONTAINERS"] = containers
	case "livekit":
		v["LIVEKIT_URL"] = url
		v["LIVEKIT_API_KEY"] = env["LIVEKIT_API_KEY"]
		v["LIVEKIT_API_SECRET"] = env["LIVEKIT_API_SECRET"]
		v["LIVEKIT_SERVER_CONTAINERS"] = containers
	case "mediasoup":
		v["MEDIASOUP_URL"] = url
		v["MEDIASOUP_ORIGIN"] = "http://" + net.JoinHostPort(address, strconv.Itoa(s.TCPPorts[0]))
		v["MEDIASOUP_SERVER_CONTAINERS"] = containers
	case "janus":
		v["JANUS_URL"] = url
		v["JANUS_SERVER_CONTAINERS"] = containers
	}
	return v
}

// writeSettings writes the file at mode 0600, also over an existing one with
// wider rights: it is the only copy of the secrets off the machine.
func writeSettings(path string, vars map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	for _, k := range slices.Sorted(maps.Keys(vars)) {
		if _, err := fmt.Fprintf(f, "%s=%s\n", k, shellQuote(vars[k])); err != nil {
			return err
		}
	}
	return f.Close()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func removeSettings(o Options) error {
	if err := os.Remove(SettingsPath(o)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("deleting the settings file: %w", err)
	}
	return nil
}
