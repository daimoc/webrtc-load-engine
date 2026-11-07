package hostload

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// sshArgs gives the ssh arguments for a DOCKER_HOST of
// ssh://[user@]host[:port]. No port opens: the Docker API stays on the
// machine's socket, reached the way the Docker CLI does (013 research R5).
func sshArgs(dockerHost string) ([]string, error) {
	u, err := url.Parse(dockerHost)
	if err != nil {
		return nil, fmt.Errorf("DOCKER_HOST: %w", err)
	}
	host := u.Hostname()
	if host == "" || strings.HasPrefix(host, "-") || strings.HasPrefix(u.User.Username(), "-") {
		return nil, errors.New("DOCKER_HOST: bad ssh host")
	}
	args := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "ServerAliveInterval=15"}
	if p := u.Port(); p != "" {
		args = append(args, "-p", p)
	}
	if name := u.User.Username(); name != "" {
		host = name + "@" + host
	}
	return append(args, "--", host, "docker", "system", "dial-stdio"), nil
}

// sshConn is a connection over the stdin and stdout of an ssh process.
type sshConn struct {
	cmd     *exec.Cmd
	in, out *os.File
	once    sync.Once
}

// dialSSH starts ssh. It takes no context: the process lives as long as the
// connection, which outlives the request that dialled it.
func dialSSH(args []string) (net.Conn, error) {
	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		inR.Close()
		inW.Close()
		return nil, err
	}
	cmd := exec.Command("ssh", args...)
	cmd.Stdin, cmd.Stdout = inR, outW
	err = cmd.Start()
	inR.Close()
	outW.Close()
	if err != nil {
		inW.Close()
		outR.Close()
		return nil, err
	}
	return &sshConn{cmd: cmd, in: inW, out: outR}, nil
}

func (c *sshConn) Read(p []byte) (int, error)  { return c.out.Read(p) }
func (c *sshConn) Write(p []byte) (int, error) { return c.in.Write(p) }

// Close ends the process and reaps it.
func (c *sshConn) Close() error {
	c.once.Do(func() {
		c.in.Close()
		c.out.Close()
		c.cmd.Process.Kill()
		c.cmd.Wait()
	})
	return nil
}

func (c *sshConn) LocalAddr() net.Addr  { return sshAddr{} }
func (c *sshConn) RemoteAddr() net.Addr { return sshAddr{} }

func (c *sshConn) SetDeadline(t time.Time) error {
	return errors.Join(c.SetReadDeadline(t), c.SetWriteDeadline(t))
}
func (c *sshConn) SetReadDeadline(t time.Time) error  { return c.out.SetReadDeadline(t) }
func (c *sshConn) SetWriteDeadline(t time.Time) error { return c.in.SetWriteDeadline(t) }

// sshAddr is the address of a connection without one.
type sshAddr struct{}

func (sshAddr) Network() string { return "ssh" }
func (sshAddr) String() string  { return "ssh" }
