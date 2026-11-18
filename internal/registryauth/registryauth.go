// Package registryauth resolves registry credentials the way the Docker CLI
// does, read-only: ~/.docker/config.json ("auths", "credsStore", "credHelpers")
// and docker-credential-<name> helpers. Secrets live only in memory, are never
// logged and never leave the Go side except in the X-Registry-Auth header.
package registryauth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/distribution/reference"

	"docklet/internal/secrets"
)

// Source values describe where credentials came from.
const (
	SourceNone   = "none"
	SourceConfig = "config"
	// SourceDocklet marks credentials kept in the OS credential store by Docklet itself.
	SourceDocklet = "docklet"
)

const (
	hubHost         = "docker.io"
	hubServerURL    = "https://index.docker.io/v1/"
	helperTimeout   = 10 * time.Second
	maxHelperOutput = 1 << 20
)

// Runner executes a credential helper. It is an interface so tests can fake it.
type Runner interface {
	// Run executes the program with args (no shell), feeding stdin, and returns stdout and stderr.
	Run(ctx context.Context, name string, stdin string, args ...string) (stdout []byte, stderr []byte, err error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name, stdin string, args ...string) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, helperTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	hideWindow(cmd)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb limitedBuffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		err = errors.New("credential helper timed out")
	}
	return out.Bytes(), errb.Bytes(), err
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len() < maxHelperOutput {
		b.Buffer.Write(p)
	}
	return len(p), nil
}

// Credentials are resolved registry credentials. Never print or log them: the
// String method redacts, and fields are only used to build the auth header.
type Credentials struct {
	Registry      string
	Source        string
	Username      string
	Password      string
	IdentityToken string
}

// Has reports whether usable credentials were found.
func (c Credentials) Has() bool {
	return c.Source != "" && c.Source != SourceNone && (c.Password != "" || c.IdentityToken != "")
}

// String redacts the secret.
func (c Credentials) String() string {
	if !c.Has() {
		return fmt.Sprintf("registry %s: no credentials", c.Registry)
	}
	return fmt.Sprintf("registry %s: credentials from %s (user %q, secret redacted)", c.Registry, c.Source, c.Username)
}

// GoString keeps %#v from leaking the secret.
func (c Credentials) GoString() string { return c.String() }

// EncodedAuth returns the value of the X-Registry-Auth header (base64url JSON)
// or "" when there are no credentials.
func (c Credentials) EncodedAuth() string {
	if !c.Has() {
		return ""
	}
	m := map[string]string{"serveraddress": c.ServerAddress()}
	if c.Username != "" {
		m["username"] = c.Username
	}
	if c.Password != "" {
		m["password"] = c.Password
	}
	if c.IdentityToken != "" {
		m["identitytoken"] = c.IdentityToken
	}
	b, _ := json.Marshal(m)
	return base64.URLEncoding.EncodeToString(b)
}

// HeaderValue is EncodedAuth, but never empty: the Engine rejects a missing
// X-Registry-Auth header on push, so anonymous requests send an empty JSON object.
func (c Credentials) HeaderValue() string {
	if v := c.EncodedAuth(); v != "" {
		return v
	}
	return base64.URLEncoding.EncodeToString([]byte("{}"))
}

// ServerAddress is the address the Engine should associate with the credentials.
func (c Credentials) ServerAddress() string {
	if c.Registry == hubHost {
		return hubServerURL
	}
	return c.Registry
}

// Resolver looks credentials up. The zero value is usable: it reads the
// default Docker config directory and executes real helpers.
type Resolver struct {
	// ConfigDir overrides the Docker config directory (default: $DOCKER_CONFIG or ~/.docker).
	ConfigDir string
	Runner    Runner
	// Store is Docklet's own credential store (OS credential store); nil disables it.
	Store secrets.Store
	// LookPath finds credential helpers (default exec.LookPath); tests replace it.
	LookPath func(string) (string, error)
}

// DefaultResolver reads the Docker config on this computer and Docklet's entries in the OS credential store.
func DefaultResolver() *Resolver { return &Resolver{Store: secrets.NewOSStore()} }

// RegistryOf returns the registry host of an image reference, with Docker Hub
// aliases (docker.io, index.docker.io, registry-1.docker.io) mapped to "docker.io".
func RegistryOf(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", errors.New("image reference is empty")
	}
	n, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return "", fmt.Errorf("invalid image reference %q", ref)
	}
	return NormalizeHost(reference.Domain(n)), nil
}

// NormalizeHost maps a registry host, URL or config key to a comparable host.
func NormalizeHost(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	switch s {
	case "index.docker.io", "registry-1.docker.io", "registry.hub.docker.com", "docker.io":
		return hubHost
	}
	return s
}

type configFile struct {
	Auths       map[string]authEntry `json:"auths"`
	CredsStore  string               `json:"credsStore"`
	CredHelpers map[string]string    `json:"credHelpers"`
}

type authEntry struct {
	Auth          string `json:"auth"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	IdentityToken string `json:"identitytoken"`
}

// helperFor returns the credential helper the Docker CLI would use for host ("" = none).
func (c configFile) helperFor(host string) string {
	for k, v := range c.CredHelpers {
		if NormalizeHost(k) == host {
			return v
		}
	}
	return c.CredsStore
}

func (r *Resolver) configPath() string {
	dir := r.ConfigDir
	if dir == "" {
		dir = os.Getenv("DOCKER_CONFIG")
	}
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".docker")
	}
	return filepath.Join(dir, "config.json")
}

func (r *Resolver) loadConfig() (configFile, error) {
	var cfg configFile
	p := r.configPath()
	if p == "" {
		return cfg, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, errors.New("cannot read the Docker config file")
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, errors.New("Docker config.json is not valid JSON")
	}
	return cfg, nil
}

// Resolve returns credentials for the registry of ref. Missing credentials are
// not an error (Source "none"). An error means credentials may exist but could
// not be read (for example a failing helper); its text never contains secrets.
func (r *Resolver) Resolve(ctx context.Context, ref string) (Credentials, error) {
	host, err := RegistryOf(ref)
	if err != nil {
		return Credentials{}, err
	}
	return r.ResolveHost(ctx, host)
}

// ResolveHost is Resolve for an already known registry host.
func (r *Resolver) ResolveHost(ctx context.Context, host string) (Credentials, error) {
	host = NormalizeHost(host)
	res := Credentials{Registry: host, Source: SourceNone}
	cfg, cfgErr := r.loadConfig()
	if cfgErr != nil {
		// An unreadable config must not hide Docklet's own sign-ins.
		if c, ok := r.fromDockletStore(host); ok {
			return c, nil
		}
		return res, cfgErr
	}
	var helperErr error
	if helper := cfg.helperFor(host); helper != "" {
		c, err := r.fromHelper(ctx, helper, host)
		if err == nil && c.Has() {
			return c, nil
		}
		helperErr = err
	}
	if c, ok := r.fromDockletStore(host); ok {
		return c, nil
	}
	for k, e := range cfg.Auths {
		if NormalizeHost(k) != host {
			continue
		}
		if c := entryToCredentials(host, e); c.Has() {
			return c, nil
		}
	}
	return res, helperErr
}

func entryToCredentials(host string, e authEntry) Credentials {
	c := Credentials{Registry: host, Source: SourceConfig, Username: e.Username, Password: e.Password, IdentityToken: e.IdentityToken}
	if e.Auth != "" {
		if dec, err := base64.StdEncoding.DecodeString(e.Auth); err == nil {
			if u, p, ok := strings.Cut(string(dec), ":"); ok {
				c.Username, c.Password = u, p
			}
		}
	}
	return c
}

var helperNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func (r *Resolver) fromHelper(ctx context.Context, name, host string) (Credentials, error) {
	res := Credentials{Registry: host, Source: SourceNone}
	user, secret, err := Helper{Name: name, Runner: r.Runner}.Get(ctx, ServerKey(host))
	if err != nil {
		return res, err
	}
	if secret == "" {
		return res, nil
	}
	c := Credentials{Registry: host, Source: "helper:" + name, Username: user, Password: secret}
	if user == "<token>" {
		c.Username, c.Password, c.IdentityToken = "", "", secret
	}
	return c, nil
}

func errText(err error) string {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return fmt.Sprintf("exit status %d", ee.ExitCode())
	}
	return err.Error()
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

func isNotFound(s string) bool {
	s = strings.ToLower(s)
	return strings.Contains(s, "credentials not found") || strings.Contains(s, "no credentials") ||
		strings.Contains(s, "not found in") || strings.Contains(s, "element not found")
}

var secretRe = regexp.MustCompile(`(?i)("?(secret|password|identitytoken|auth|token)"?\s*[:=]\s*)("[^"]*"|\S+)`)

// Redact masks anything that looks like a secret assignment in s.
func Redact(s string) string {
	return secretRe.ReplaceAllString(s, `${1}"[redacted]"`)
}
