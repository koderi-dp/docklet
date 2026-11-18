package engine

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"

	"docklet/internal/model"
)

// DiscoveredLocalID is the stable ID of the auto-discovered local profile.
const DiscoveredLocalID = "local-default"

// DefaultLocalProfile is the platform default local endpoint, flagged Discovered.
func DefaultLocalProfile() model.ConnectionProfile {
	if runtime.GOOS == "windows" {
		return model.ConnectionProfile{ID: DiscoveredLocalID, Name: "Local Docker Engine", Type: model.ConnNamedPipe,
			Endpoint: "npipe:////./pipe/docker_engine", Discovered: true}
	}
	return model.ConnectionProfile{ID: DiscoveredLocalID, Name: "Local Docker Engine", Type: model.ConnUnix,
		Endpoint: "unix:///var/run/docker.sock", Discovered: true}
}

var (
	safeToken  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	safeDistro = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._ -]*$`)
)

// ValidWSLDistroName reports whether name is safe to pass as a WSL distro argument.
func ValidWSLDistroName(name string) bool { return safeDistro.MatchString(name) }

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// UsesTLS reports whether a tcp profile is TLS-protected (always verified).
func UsesTLS(p model.ConnectionProfile) bool {
	return p.TLSVerify || strings.TrimSpace(p.TLSCertDir) != ""
}

// ValidateProfile checks a profile and returns a normalized copy (trimmed,
// Discovered cleared). It does not assign an ID.
func ValidateProfile(p model.ConnectionProfile) (model.ConnectionProfile, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.Endpoint = strings.TrimSpace(p.Endpoint)
	p.WSLDistro = strings.TrimSpace(p.WSLDistro)
	p.TLSCertDir = strings.TrimSpace(p.TLSCertDir)
	p.Discovered = false
	if p.Type != model.ConnTCP {
		p.TLSVerify, p.TLSCertDir, p.AllowInsecure = false, "", false
	}
	if p.Type != model.ConnWSL {
		p.WSLDistro = ""
	}
	if p.Type != model.ConnSSH {
		p.SSHUser, p.SSHPort, p.SSHIdentityFile, p.SSHUseAgent, p.SSHProxyJump = "", 0, "", false, ""
	}
	if p.Name == "" {
		return p, errors.New("connection name is required")
	}
	if len(p.Name) > 100 {
		return p, errors.New("connection name is too long")
	}
	switch p.Type {
	case model.ConnNamedPipe, model.ConnUnix:
		u, err := parseEndpoint(p.Endpoint, string(p.Type)+"://")
		if err != nil {
			return p, err
		}
		if u.Path == "" && u.Host == "" {
			return p, fmt.Errorf("endpoint %q has no path", p.Endpoint)
		}
	case model.ConnTCP:
		u, err := parseEndpoint(p.Endpoint, "tcp://")
		if err != nil {
			return p, err
		}
		if u.Hostname() == "" || u.Port() == "" {
			return p, errors.New("tcp endpoint must be tcp://host:port")
		}
		if u.Path != "" && u.Path != "/" {
			return p, errors.New("tcp endpoint must not contain a path")
		}
		if dir := p.TLSCertDir; dir != "" {
			if st, err := os.Stat(dir); err != nil || !st.IsDir() {
				return p, fmt.Errorf("certificate directory %q was not found", dir)
			}
		}
		if UsesTLS(p) {
			p.AllowInsecure = false // TLS is verified; the opt-in is meaningless
		} else if !isLoopback(u.Hostname()) && !p.AllowInsecure {
			return p, errors.New("plain (unencrypted) tcp to a non-local host is not allowed: enable TLS verification, or explicitly allow insecure plain TCP")
		}
	case model.ConnSSH:
		u, err := parseEndpoint(p.Endpoint, "ssh://")
		if err != nil {
			return p, err
		}
		if u.User != nil {
			if _, hasPw := u.User.Password(); hasPw {
				return p, errors.New("endpoint must not contain a password; use an SSH key or agent")
			}
			if !safeToken.MatchString(u.User.Username()) {
				return p, errors.New("invalid ssh user name")
			}
		}
		if !safeToken.MatchString(u.Hostname()) {
			return p, errors.New("invalid ssh host")
		}
		if u.Path != "" && u.Path != "/" {
			return p, errors.New("ssh endpoint must not contain a path")
		}
		if err := validateSSHFields(&p); err != nil {
			return p, err
		}
	case model.ConnWSL:
		if p.WSLDistro == "" {
			return p, errors.New("WSL distribution name is required")
		}
		if !safeDistro.MatchString(p.WSLDistro) {
			return p, errors.New("invalid WSL distribution name")
		}
	case model.ConnContext:
		// Accepted for storage; connecting reports "not supported yet".
	default:
		return p, fmt.Errorf("unknown connection type %q", p.Type)
	}
	return p, nil
}

func parseEndpoint(endpoint, wantPrefix string) (*url.URL, error) {
	if endpoint == "" {
		return nil, errors.New("endpoint is required")
	}
	if !strings.HasPrefix(strings.ToLower(endpoint), wantPrefix) {
		return nil, fmt.Errorf("endpoint must start with %s for this connection type", wantPrefix)
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.New("endpoint is not a valid URL")
	}
	if wantPrefix != "ssh://" && u.User != nil {
		return nil, errors.New("endpoint must not contain credentials")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("endpoint must not contain a query or fragment")
	}
	return u, nil
}
