package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/moby/moby/client"

	"docklet/internal/model"
	"docklet/internal/secrets"
)

// clientFactory builds an SDK client for a profile. Overridable in tests.
type clientFactory func(p model.ConnectionProfile) (*client.Client, error)

// newSDKClient routes a profile to its transport. The returned client has no
// global request timeout (it would break streaming); callers bound calls via ctx.
func newSDKClient(p model.ConnectionProfile) (*client.Client, error) {
	return newSDKClientWith(p, nil)
}

// newSDKClientWith is newSDKClient with a secret store for SSH password/passphrase auth.
func newSDKClientWith(p model.ConnectionProfile, st secrets.Store) (*client.Client, error) {
	common := []client.Opt{client.WithAPIVersionNegotiation()}
	switch p.Type {
	case model.ConnNamedPipe, model.ConnUnix:
		return client.New(append(common, client.WithHost(p.Endpoint))...)
	case model.ConnTCP:
		opts := append(common, client.WithHost(p.Endpoint))
		if UsesTLS(p) {
			ca, cert, key := tlsFiles(p.TLSCertDir)
			opts = append(opts, client.WithTLSClientConfig(ca, cert, key))
		}
		return client.New(opts...)
	case model.ConnWSL:
		args := []string{"-d", p.WSLDistro, "--exec", "docker", "system", "dial-stdio"}
		return client.New(append(common,
			client.WithHost("http://docker.example.com"),
			client.WithDialContext(stdioDialer("wsl.exe", args)))...)
	case model.ConnSSH:
		dial, err := sshDialer(p, st)
		if err != nil {
			return nil, err
		}
		return client.New(append(common,
			client.WithHost("http://docker.example.com"),
			client.WithDialContext(dial))...)
	case model.ConnContext:
		return nil, errors.New("Docker contexts are not supported yet; import the context as a tcp/ssh/npipe connection")
	}
	return nil, fmt.Errorf("unknown connection type %q", p.Type)
}

// tlsFiles returns ca/cert/key paths from a Docker-style cert dir; only files
// that exist are returned (client cert needs both cert and key).
func tlsFiles(dir string) (ca, cert, key string) {
	if dir == "" {
		return
	}
	exists := func(n string) string {
		f := filepath.Join(dir, n)
		if _, err := os.Stat(f); err == nil {
			return f
		}
		return ""
	}
	ca = exists("ca.pem")
	cert, key = exists("cert.pem"), exists("key.pem")
	if cert == "" || key == "" {
		cert, key = "", ""
	}
	return
}

// stdioDialer returns a dialer that spawns `name args...` and speaks the Engine
// API over its stdin/stdout (`docker system dial-stdio`). No listener is opened.
func stdioDialer(name string, args []string) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		cmd := exec.Command(name, args...)
		hideWindow(cmd)
		return startCmdConn(cmd)
	}
}

type cmdAddr struct{}

func (cmdAddr) Network() string { return "stdio" }
func (cmdAddr) String() string  { return "stdio" }

type limitedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *limitedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if room := 4096 - l.b.Len(); room > 0 {
		if len(p) < room {
			room = len(p)
		}
		l.b.Write(p[:room])
	}
	return len(p), nil
}

func (l *limitedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.TrimSpace(l.b.String())
}

// cmdConn adapts a child process's stdio to net.Conn.
type cmdConn struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr *limitedBuf
	once   sync.Once
	// onClose runs once when the connection is closed (e.g. stops the askpass server).
	onClose func()
	// gotData is set once any stdout byte was read (an early EOF is a failed start).
	gotData atomic.Bool
}

func startCmdConn(cmd *exec.Cmd) (*cmdConn, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	c := &cmdConn{cmd: cmd, stdin: stdin, stdout: stdout, stderr: &limitedBuf{}}
	cmd.Stderr = c.stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *cmdConn) Read(p []byte) (int, error) {
	n, err := c.stdout.Read(p)
	if n > 0 {
		c.gotData.Store(true)
	}
	if err != nil && n == 0 && !c.gotData.Load() {
		// The child's stderr is copied asynchronously; give it a moment so the
		// real reason (ssh/wsl message) is attached to the EOF.
		for i := 0; i < 10; i++ {
			if msg := c.stderr.String(); msg != "" {
				return 0, fmt.Errorf("%w: %s", err, msg)
			}
			time.Sleep(30 * time.Millisecond)
		}
	}
	return n, err
}

func (c *cmdConn) Write(p []byte) (int, error) { return c.stdin.Write(p) }

func (c *cmdConn) Close() error {
	c.once.Do(func() {
		_ = c.stdin.Close()
		if c.onClose != nil {
			defer c.onClose()
		}
		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		go func() { _ = c.cmd.Wait() }()
	})
	return nil
}

func (c *cmdConn) LocalAddr() net.Addr              { return cmdAddr{} }
func (c *cmdConn) RemoteAddr() net.Addr             { return cmdAddr{} }
func (c *cmdConn) SetDeadline(time.Time) error      { return nil }
func (c *cmdConn) SetReadDeadline(time.Time) error  { return nil }
func (c *cmdConn) SetWriteDeadline(time.Time) error { return nil }

// composeAvailable reports whether `docker compose version` runs for this
// profile's environment. Only local transports and WSL are probed.
func composeAvailable(ctx context.Context, p model.ConnectionProfile) (bool, string) {
	var cmd *exec.Cmd
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	switch p.Type {
	case model.ConnNamedPipe, model.ConnUnix:
		cmd = exec.CommandContext(cctx, "docker", "compose", "version")
	case model.ConnWSL:
		cmd = exec.CommandContext(cctx, "wsl.exe", "-d", p.WSLDistro, "--exec", "docker", "compose", "version")
	default:
		return false, "Compose execution is only checked for local and WSL connections."
	}
	hideWindow(cmd)
	if err := cmd.Run(); err != nil {
		if p.Type == model.ConnWSL {
			return false, "docker compose is not available in the WSL distribution."
		}
		return false, "docker compose is not installed or not runnable on this machine."
	}
	return true, ""
}
