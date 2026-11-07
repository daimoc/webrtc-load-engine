package remote

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	remoteRoot = `"$HOME/sfu-remote`
	overlay    = "docker-compose.remote.yml"

	stateNone    = "none"
	stateRunning = "running"
	stateStopped = "stopped"
	stateFailed  = "failed"

	accessPrivate  = "private"
	accessAllow    = "allow"
	accessExternal = "external"
)

// Options configures the commands. The zero values of the last fields are
// what the command line uses.
type Options struct {
	Target           Target
	Address          string         // announced address; default: the host when it is an IP
	Allow            []netip.Prefix // CIDRs reaching the server's ports
	ExternalFirewall bool           // the operator states one exists
	PlatformsDir     string         // default tests/platforms
	SettingsDir      string         // default: the user's config folder
	Log              io.Writer      // progress; default none

	Poll time.Duration           // between checks while waiting; default 2 s
	Wait time.Duration           // for the server once started; default 10 min
	Dial func(addr string) error // TCP probe of status; default a real connection
}

func (o Options) logf(format string, args ...any) {
	if o.Log != nil {
		fmt.Fprintf(o.Log, format+"\n", args...)
	}
}

func (o Options) platformsDir() string {
	if o.PlatformsDir != "" {
		return o.PlatformsDir
	}
	return filepath.Join("tests", "platforms")
}

func path(parts ...string) string {
	return remoteRoot + "/" + strings.Join(parts, "/") + `"`
}

// writeFile writes stdin to a file on the machine, readable by the user only.
func writeFile(ctx context.Context, r Runner, target string, content []byte) error {
	cmd := "(umask 077; cat > " + target + ") && chmod 600 " + target
	if _, err := r.Run(ctx, cmd, bytes.NewReader(content)); err != nil {
		return fmt.Errorf("writing %s: %w", target, err)
	}
	return nil
}

func readFile(ctx context.Context, r Runner, target string) ([]byte, error) {
	return r.Run(ctx, "cat "+target+" 2>/dev/null; true", nil)
}

// access is what <server>/access records.
type access struct {
	mode, address string
	allowed       []netip.Prefix
}

func (a access) bytes() []byte {
	cidrs := make([]string, len(a.allowed))
	for i, p := range a.allowed {
		cidrs[i] = p.String()
	}
	return []byte("mode=" + a.mode + "\naddress=" + a.address + "\nallowed=" + strings.Join(cidrs, ",") + "\n")
}

func parseAccess(b []byte) access {
	v := ParseEnv(b)
	a := access{mode: v["mode"], address: v["address"]}
	for _, c := range strings.Split(v["allowed"], ",") {
		if p, err := netip.ParsePrefix(c); err == nil {
			a.allowed = append(a.allowed, p)
		}
	}
	return a
}

func (a access) String() string {
	if a.mode != accessAllow {
		return a.mode
	}
	cidrs := make([]string, len(a.allowed))
	for i, p := range a.allowed {
		cidrs[i] = p.String()
	}
	return a.mode + " " + strings.Join(cidrs, " ")
}

// current returns the deployed server's name, empty when there is none.
func current(ctx context.Context, r Runner) (string, error) {
	b, err := readFile(ctx, r, path("current"))
	if err != nil {
		return "", fmt.Errorf("reading the deployment: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

func loadAccess(ctx context.Context, r Runner, name string) (access, error) {
	b, err := readFile(ctx, r, path(name, "access"))
	if err != nil {
		return access{}, fmt.Errorf("reading the deployment: %w", err)
	}
	return parseAccess(b), nil
}

// container is what docker inspect says of one container.
type container struct {
	status, image, project string
}

// inspect reads the server's containers. One that does not exist is absent.
func inspect(ctx context.Context, r Runner, s Server) (map[string]container, error) {
	names := slices.Sorted(maps.Keys(s.Images))
	cmd := `docker inspect --format '{{.Name}}|{{.State.Status}}|{{.Config.Image}}|{{index .Config.Labels "com.docker.compose.project"}}' ` +
		strings.Join(names, " ") + " 2>/dev/null; true"
	out, err := r.Run(ctx, cmd, nil)
	if err != nil {
		return nil, fmt.Errorf("reading containers: %w", err)
	}
	found := map[string]container{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.SplitN(line, "|", 4)
		if len(f) == 4 {
			found[strings.TrimPrefix(f[0], "/")] = container{f[1], f[2], f[3]}
		}
	}
	return found, nil
}

// resolveAddress gives the address servers announce.
func resolveAddress(o Options) (netip.Addr, error) {
	text := o.Address
	if text == "" {
		text = o.Target.Host
	}
	addr, err := netip.ParseAddr(text)
	if err != nil {
		if o.Address != "" {
			return netip.Addr{}, fmt.Errorf("--address %s: not an IP address", o.Address)
		}
		return netip.Addr{}, fmt.Errorf("%s is not an IP address: give --address", o.Target.Host)
	}
	return addr, nil
}

// Bundle tars a setup folder. Besides .env, private keys stay behind: a local
// certificate's key must not leave the workstation (research R2).
func Bundle(dir string) ([]byte, error) {
	if _, err := os.Stat(filepath.Join(dir, overlay)); err != nil {
		return nil, fmt.Errorf("the setup has no %s: %w", overlay, err)
	}
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if !d.Type().IsRegular() || name == ".env" || strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".key") {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		h := &tar.Header{Name: filepath.ToSlash(rel), Mode: int64(info.Mode().Perm()), Size: info.Size(), ModTime: info.ModTime(), Typeflag: tar.TypeReg}
		if err := w.WriteHeader(h); err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Deploy starts server s on the machine. It checks everything before changing
// anything, and is safe to run again (FR-005).
func Deploy(ctx context.Context, r Runner, s Server, o Options) (*Report, error) {
	warnings, err := Check(ctx, r, s, o)
	if err != nil {
		return nil, err
	}
	for _, w := range warnings {
		o.logf("warning: %s", w)
	}
	ipOut, err := r.Run(ctx, "ip -j addr", nil)
	if err != nil {
		return nil, fmt.Errorf("reading the machine's addresses: %w", err)
	}
	addrs, err := Classify(ipOut)
	if err != nil {
		return nil, err
	}
	a := access{mode: accessPrivate, allowed: o.Allow}
	switch {
	case len(o.Allow) > 0:
		a.mode = accessAllow
	case o.ExternalFirewall:
		a.mode = accessExternal
	case len(addrs.Public) > 0:
		return nil, fmt.Errorf("%s is a public address, and the servers listen on every interface.\n"+
			"Give --allow <your-cidr>, or --external-firewall if a firewall already restricts it.", addrs.Public[0])
	}
	addr, err := resolveAddress(o)
	if err != nil {
		return nil, err
	}
	a.address = addr.String()

	found, err := inspect(ctx, r, s)
	if err != nil {
		return nil, err
	}
	for _, name := range slices.Sorted(maps.Keys(found)) {
		if found[name].project != s.project() {
			return nil, fmt.Errorf("a container named %s exists and is not from this project: remove it first", name)
		}
	}
	bundle, err := Bundle(filepath.Join(o.platformsDir(), s.Folder))
	if err != nil {
		return nil, err
	}
	example, err := os.ReadFile(filepath.Join(o.platformsDir(), s.Folder, "env.example"))
	if err != nil {
		return nil, err
	}

	cur, err := current(ctx, r)
	if err != nil {
		return nil, err
	}
	if cur != "" && cur != s.Name {
		o.logf("removing %s", cur)
		if err := Remove(ctx, r, o); err != nil {
			return nil, fmt.Errorf("removing %s: %w", cur, err)
		}
	}
	if cur == s.Name {
		// Folder and secrets stay. Without this, a container recreated for
		// new settings leaves its old anonymous volume where remove cannot
		// see it.
		o.logf("stopping %s", s.Name)
		if _, err := r.Run(ctx, "docker compose -p "+s.project()+" down -v --remove-orphans", nil); err != nil {
			return nil, fmt.Errorf("stopping %s: %w", s.Name, err)
		}
	}
	secrets, err := secretsFor(ctx, r, s)
	if err != nil {
		return nil, err
	}

	// The marker comes first, so that remove can clean up a failed deployment.
	dir := path(s.Name)
	if _, err := r.Run(ctx, "mkdir -p "+dir, nil); err != nil {
		return nil, fmt.Errorf("creating the deployment folder: %w", err)
	}
	if err := writeFile(ctx, r, path("current"), []byte(s.Name+"\n")); err != nil {
		return nil, err
	}
	if err := writeFile(ctx, r, path(s.Name, "access"), a.bytes()); err != nil {
		return nil, err
	}
	o.logf("sending %s", s.Folder)
	if _, err := r.Run(ctx, "tar -xf - -C "+dir, bytes.NewReader(bundle)); err != nil {
		return nil, fmt.Errorf("sending the setup: %w", err)
	}
	env := RenderEnv(example, Values(s, a.address, secrets, addrs.Interfaces[addr]))
	if err := writeFile(ctx, r, path(s.Name, ".env"), env); err != nil {
		return nil, err
	}
	if s.Name == "jitsi" {
		// Only once: on a second run containers own these folders.
		if _, err := r.Run(ctx, "cd "+dir+" && { [ -d config/tmp/web-load-test ] || ./setup.sh; }", nil); err != nil {
			return nil, fmt.Errorf("preparing Jitsi: %w", err)
		}
	}
	if a.mode == accessAllow {
		o.logf("installing firewall rules")
		if _, err := r.Run(ctx, strings.Join(FirewallCommands(s, a.allowed, addrs.hasPublicIPv6()), " && "), nil); err != nil {
			return nil, fmt.Errorf("installing firewall rules: %w", err)
		}
	}
	o.logf("starting %s (a first build takes a while)", s.Name)
	up := "cd " + dir + " && docker compose -p " + s.project() + " -f docker-compose.yml -f " + overlay +
		" up -d --build " + strings.Join(s.Services, " ")
	if _, err := r.Run(ctx, up, nil); err != nil {
		return nil, fmt.Errorf("starting %s: %w", s.Name, err)
	}
	if err := waitRunning(ctx, r, s, o); err != nil {
		return nil, err
	}
	if err := checkPins(ctx, r, s); err != nil {
		if rerr := Remove(ctx, r, o); rerr != nil {
			err = errors.Join(err, rerr)
		}
		return nil, err
	}
	if err := writeSettings(SettingsPath(o), settingsVars(s, o.Target, a.address, ParseEnv(env))); err != nil {
		return nil, fmt.Errorf("writing the settings file: %w", err)
	}
	return Status(ctx, r, o)
}

// secretsFor keeps the secrets of a deployment of s, so that a retry does not
// invalidate the workstation's settings, and generates those missing.
func secretsFor(ctx context.Context, r Runner, s Server) (map[string]string, error) {
	fresh, err := NewSecrets(s)
	if err != nil {
		return nil, err
	}
	b, err := readFile(ctx, r, path(s.Name, ".env"))
	if err != nil {
		return nil, fmt.Errorf("reading the deployment: %w", err)
	}
	old := ParseEnv(b)
	for k := range fresh {
		if old[k] != "" {
			fresh[k] = old[k]
		}
	}
	return fresh, nil
}

// waitRunning waits for all containers and the signaling port.
func waitRunning(ctx context.Context, r Runner, s Server, o Options) error {
	poll, wait := o.Poll, o.Wait
	if poll == 0 {
		poll = 2 * time.Second
	}
	if wait == 0 {
		wait = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	probe := fmt.Sprintf("timeout 2 bash -c '</dev/tcp/127.0.0.1/%d'", s.TCPPorts[0])
	for {
		waiting, err := notReady(ctx, r, s, probe)
		if err == nil && waiting == "" {
			return nil
		}
		var stopped stoppedError
		if errors.As(err, &stopped) {
			return fmt.Errorf("%s did not start: %w", s.Name, err)
		}
		select {
		case <-ctx.Done():
			if waiting == "" {
				waiting = fmt.Sprintf("%v", err)
			}
			return fmt.Errorf("%s did not start: %s", s.Name, waiting)
		case <-time.After(poll):
		}
	}
}

// stoppedError is a container that cannot come back: the overlays set
// restart "no". It points at the logs rather than quoting them, since a
// server's logs may hold a secret.
type stoppedError struct {
	container, status string
}

func (e stoppedError) Error() string {
	return fmt.Sprintf("%s %s: read why with docker logs %s on the machine", e.container, e.status, e.container)
}

// notReady names what is not up yet, empty when all is.
func notReady(ctx context.Context, r Runner, s Server, probe string) (string, error) {
	found, err := inspect(ctx, r, s)
	if err != nil {
		return "", err
	}
	var missing []string
	for _, name := range slices.Sorted(maps.Keys(s.Images)) {
		if st := found[name].status; st == "exited" || st == "dead" {
			return "", stoppedError{name, st}
		}
		if found[name].status != "running" {
			missing = append(missing, fmt.Sprintf("%s (%s)", name, orAbsent(found[name].status)))
		}
	}
	if len(missing) > 0 {
		return strings.Join(missing, ", "), nil
	}
	if _, err := r.Run(ctx, probe, nil); err != nil {
		return fmt.Sprintf("port %d", s.TCPPorts[0]), nil
	}
	if s.Ready != "" {
		if _, err := r.Run(ctx, s.Ready, nil); err != nil {
			return s.Waiting, nil
		}
	}
	return "", nil
}

func orAbsent(status string) string {
	if status == "" {
		return "absent"
	}
	return status
}

// checkPins refuses a version other than the pinned one (spec edge case).
func checkPins(ctx context.Context, r Runner, s Server) error {
	found, err := inspect(ctx, r, s)
	if err != nil {
		return err
	}
	for _, name := range slices.Sorted(maps.Keys(s.Images)) {
		if got, want := found[name].image, s.Images[name]; got != want {
			return fmt.Errorf("%s runs %s, the pinned version is %s", name, got, want)
		}
	}
	return nil
}

// Report is what status prints. It holds no secret.
type Report struct {
	Target    Target
	Server    string
	State     string
	Access    string
	Address   string
	URL       string
	Settings  string
	TLS       string
	Reachable []string
}

func (r *Report) String() string {
	var b strings.Builder
	line := func(key, value string) { fmt.Fprintf(&b, "%-11s%s\n", key+":", value) }
	line("host", r.Target.String())
	if r.Server == "" {
		line("server", stateNone)
		return b.String()
	}
	line("server", r.Server+" ("+r.State+")")
	line("access", r.Access)
	line("address", r.Address)
	line("url", r.URL)
	line("settings", r.Settings)
	line("tls", r.TLS)
	for i, p := range r.Reachable {
		if i == 0 {
			line("reachable", p)
		} else {
			fmt.Fprintf(&b, "%11s%s\n", "", p)
		}
	}
	return b.String()
}

// Status reports the deployment. It reports, it does not judge: unreachable
// ports are named and are not an error.
func Status(ctx context.Context, r Runner, o Options) (*Report, error) {
	rep := &Report{Target: o.Target, State: stateNone}
	name, err := current(ctx, r)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return rep, nil
	}
	s, ok := LookupServer(name)
	if !ok {
		return nil, fmt.Errorf("the machine runs an unknown server %q", name)
	}
	rep.Server = name
	a, err := loadAccess(ctx, r, name)
	if err != nil {
		return nil, err
	}
	rep.Access, rep.Address = a.String(), a.address
	rep.URL, rep.Settings, rep.TLS = serverURL(s, a.address), SettingsPath(o), tlsNote(s, serverURL(s, a.address))
	found, err := inspect(ctx, r, s)
	if err != nil {
		return nil, err
	}
	rep.State = deploymentState(s, found)
	dial := o.Dial
	if dial == nil {
		dial = dialTCP
	}
	for _, p := range s.TCPPorts {
		state := "ok"
		if err := dial(net.JoinHostPort(a.address, strconv.Itoa(p))); err != nil {
			state = "unreachable"
		}
		rep.Reachable = append(rep.Reachable, fmt.Sprintf("%d/tcp %s", p, state))
	}
	rep.Reachable = append(rep.Reachable, "UDP ports cannot be probed: a bot run shows them")
	return rep, nil
}

func dialTCP(addr string) error {
	c, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return err
	}
	return c.Close()
}

// deploymentState tells a server stopped by a reboot, whose containers all
// exist and have exited, from one that failed.
func deploymentState(s Server, found map[string]container) string {
	running, exited := 0, 0
	for name := range s.Images {
		switch found[name].status {
		case "running":
			running++
		case "exited":
			exited++
		}
	}
	switch {
	case running == len(s.Images):
		return stateRunning
	case exited == len(s.Images):
		return stateStopped
	}
	return stateFailed
}

// Allow replaces the allowed addresses while a server runs.
func Allow(ctx context.Context, r Runner, allowed []netip.Prefix, o Options) error {
	name, err := current(ctx, r)
	if err != nil {
		return err
	}
	if name == "" {
		return errors.New("nothing is deployed")
	}
	a, err := loadAccess(ctx, r, name)
	if err != nil {
		return err
	}
	if a.mode != accessAllow {
		return fmt.Errorf("%s was not deployed with --allow", name)
	}
	ipOut, err := r.Run(ctx, "ip -j addr", nil)
	if err != nil {
		return fmt.Errorf("reading the machine's addresses: %w", err)
	}
	addrs, err := Classify(ipOut)
	if err != nil {
		return err
	}
	if _, err := r.Run(ctx, strings.Join(firewallBody(allowed, addrs.hasPublicIPv6()), " && "), nil); err != nil {
		return fmt.Errorf("updating firewall rules: %w", err)
	}
	a.allowed = allowed
	return writeFile(ctx, r, path(name, "access"), a.bytes())
}

// Remove stops what the deployment started and nothing else: its Compose
// project, its folder, its rules. Images stay, for the next deployment.
// Removing when nothing is deployed succeeds.
func Remove(ctx context.Context, r Runner, o Options) error {
	name, err := current(ctx, r)
	if err != nil {
		return err
	}
	if name == "" {
		return removeSettings(o)
	}
	s, ok := LookupServer(name)
	if !ok {
		return fmt.Errorf("the machine runs an unknown server %q", name)
	}
	a, err := loadAccess(ctx, r, name)
	if err != nil {
		return err
	}
	if _, err := r.Run(ctx, "docker compose -p "+s.project()+" down -v --remove-orphans", nil); err != nil {
		return fmt.Errorf("stopping %s: %w", name, err)
	}
	// Rules go after the server has stopped: never a moment without them.
	if a.mode == accessAllow {
		if _, err := r.Run(ctx, strings.Join(RemoveFirewallCommands(s), "; "), nil); err != nil {
			return fmt.Errorf("removing firewall rules: %w", err)
		}
	}
	// Containers write files as other users, which rm cannot delete.
	image := s.Images[slices.Sorted(maps.Keys(s.Images))[0]]
	rm := "rm -rf " + path(name) + " 2>/dev/null || docker run --rm -v " + remoteRoot + `:/d" --entrypoint sh ` + image + " -c 'rm -rf /d/" + name + "'"
	if _, err := r.Run(ctx, rm, nil); err != nil {
		return fmt.Errorf("deleting the deployment: %w", err)
	}
	if _, err := r.Run(ctx, "rm -f "+path("current"), nil); err != nil {
		return fmt.Errorf("deleting the deployment: %w", err)
	}
	return removeSettings(o)
}
