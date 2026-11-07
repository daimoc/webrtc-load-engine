// Command sfu-remote deploys a media server of tests/platforms to a remote
// Linux machine over SSH (specs/013-remote-sfu-deployment).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/signal"
	"syscall"

	"webrtc-load-engine/src/remote"
)

const usage = `usage:
  sfu-remote deploy <jitsi|livekit|mediasoup|janus> --host <target> [flags]
  sfu-remote status --host <target>
  sfu-remote allow  --host <target> --allow <cidr> [--allow <cidr>...]
  sfu-remote remove --host <target>`

const (
	exitOK     = 0
	exitFailed = 1
	exitUsage  = 2
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr, func(t remote.Target) remote.Runner {
		return remote.SSHRunner{Target: t}
	}))
}

// repeated collects a repeatable flag.
type repeated []string

func (r *repeated) String() string     { return fmt.Sprint([]string(*r)) }
func (r *repeated) Set(v string) error { *r = append(*r, v); return nil }

var errUsage = errors.New("usage")

// parsed is a command line.
type parsed struct {
	command string
	server  remote.Server
	opts    remote.Options
}

func parse(args []string, stderr io.Writer) (parsed, error) {
	var p parsed
	if len(args) == 0 {
		return p, errUsage
	}
	p.command, args = args[0], args[1:]
	switch p.command {
	case "deploy":
		if len(args) == 0 {
			return p, errUsage
		}
		s, ok := remote.LookupServer(args[0])
		if !ok {
			return p, fmt.Errorf("unknown server %q", args[0])
		}
		p.server, args = s, args[1:]
	case "status", "allow", "remove":
	default:
		return p, fmt.Errorf("unknown command %q", p.command)
	}
	fs := flag.NewFlagSet(p.command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	host := fs.String("host", "", "[user@]host[:port], or an ~/.ssh/config alias")
	var allow repeated
	if p.command == "deploy" || p.command == "allow" {
		fs.Var(&allow, "allow", "CIDR reaching the server's ports; repeatable")
	}
	if p.command == "deploy" {
		fs.StringVar(&p.opts.Address, "address", "", "address the server announces, when it differs from the host")
		fs.BoolVar(&p.opts.ExternalFirewall, "external-firewall", false, "a firewall outside the machine restricts access")
		fs.StringVar(&p.opts.PlatformsDir, "platforms-dir", "", "default tests/platforms")
	}
	if err := fs.Parse(args); err != nil {
		return p, err
	}
	if fs.NArg() > 0 {
		return p, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	t, err := remote.ParseTarget(*host)
	if err != nil {
		return p, err
	}
	p.opts.Target = t
	if len(allow) > 0 || p.command == "allow" {
		var prefixes []netip.Prefix
		if prefixes, err = remote.ParseAllowed(allow); err != nil {
			return p, err
		}
		p.opts.Allow = prefixes
	}
	return p, nil
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, newRunner func(remote.Target) remote.Runner) int {
	p, err := parse(args, stderr)
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stderr, err)
		}
		fmt.Fprintln(stderr, usage)
		return exitUsage
	}
	p.opts.Log = stderr
	r := newRunner(p.opts.Target)
	var report *remote.Report
	switch p.command {
	case "deploy":
		report, err = remote.Deploy(ctx, r, p.server, p.opts)
	case "status":
		report, err = remote.Status(ctx, r, p.opts)
	case "allow":
		err = remote.Allow(ctx, r, p.opts.Allow, p.opts)
	case "remove":
		err = remote.Remove(ctx, r, p.opts)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitFailed
	}
	if report != nil {
		fmt.Fprint(stdout, report)
	}
	return exitOK
}
