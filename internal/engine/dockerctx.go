package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"docklet/internal/model"
)

// dockerConfigDir returns the Docker CLI config dir (DOCKER_CONFIG or ~/.docker).
func dockerConfigDir() string {
	if d := os.Getenv("DOCKER_CONFIG"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".docker")
}

type ctxMeta struct {
	Name     string `json:"Name"`
	Metadata struct {
		Description string `json:"Description"`
	} `json:"Metadata"`
	Endpoints map[string]struct {
		Host          string `json:"Host"`
		SkipTLSVerify bool   `json:"SkipTLSVerify"`
	} `json:"Endpoints"`
}

func contextTypeFor(endpoint string) (model.ConnectionType, string) {
	l := strings.ToLower(endpoint)
	switch {
	case strings.HasPrefix(l, "npipe://"):
		return model.ConnNamedPipe, ""
	case strings.HasPrefix(l, "unix://"):
		return model.ConnUnix, ""
	case strings.HasPrefix(l, "tcp://"):
		return model.ConnTCP, ""
	case strings.HasPrefix(l, "ssh://"):
		return model.ConnSSH, ""
	case endpoint == "":
		return "", "This context has no Docker endpoint."
	}
	return "", "Unsupported endpoint scheme."
}

// readDockerContexts lists contexts under a docker config dir. Read-only.
func readDockerContexts(dir string) ([]model.DockerContextInfo, error) {
	out := []model.DockerContextInfo{}
	if dir == "" {
		return out, nil
	}
	current := os.Getenv("DOCKER_CONTEXT")
	if current == "" {
		if b, err := os.ReadFile(filepath.Join(dir, "config.json")); err == nil {
			var cfg struct {
				CurrentContext string `json:"currentContext"`
			}
			if json.Unmarshal(b, &cfg) == nil {
				current = cfg.CurrentContext
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "contexts", "meta"))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, "contexts", "meta", e.Name(), "meta.json"))
		if err != nil {
			continue
		}
		var m ctxMeta
		if json.Unmarshal(b, &m) != nil || m.Name == "" {
			continue
		}
		host := m.Endpoints["docker"].Host
		t, reason := contextTypeFor(host)
		out = append(out, model.DockerContextInfo{
			Name: m.Name, Endpoint: host, Current: m.Name == current,
			Description: m.Metadata.Description, Type: t, Reason: reason,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// contextProfile maps a context to an (unsaved) connection profile of the
// matching type. TLS material stored with the context is referenced, never copied.
func contextProfile(dir, name string) (model.ConnectionProfile, error) {
	list, err := readDockerContexts(dir)
	if err != nil {
		return model.ConnectionProfile{}, err
	}
	for _, c := range list {
		if c.Name != name {
			continue
		}
		if c.Type == "" {
			return model.ConnectionProfile{}, fmt.Errorf("context %q cannot be imported: %s", name, c.Reason)
		}
		p := model.ConnectionProfile{Name: name + " (context)", Type: c.Type, Endpoint: c.Endpoint}
		if c.Type == model.ConnTCP {
			sum := sha256.Sum256([]byte(name))
			tlsDir := filepath.Join(dir, "contexts", "tls", hex.EncodeToString(sum[:]), "docker")
			if st, err := os.Stat(tlsDir); err == nil && st.IsDir() {
				p.TLSCertDir, p.TLSVerify = tlsDir, true
			}
		}
		return p, nil
	}
	return model.ConnectionProfile{}, fmt.Errorf("docker context %q was not found", name)
}

// DockerContexts lists Docker CLI contexts (read-only).
func (m *Manager) DockerContexts(ctx context.Context) ([]model.DockerContextInfo, error) {
	return readDockerContexts(dockerConfigDir())
}

// ContextProfile returns the unsaved profile an import of the context would create.
func (m *Manager) ContextProfile(ctx context.Context, name string) (model.ConnectionProfile, error) {
	return contextProfile(dockerConfigDir(), name)
}

// ImportContext saves a connection profile created from a Docker context.
func (m *Manager) ImportContext(ctx context.Context, name string) (model.ConnectionProfile, error) {
	p, err := m.ContextProfile(ctx, name)
	if err != nil {
		return model.ConnectionProfile{}, err
	}
	return m.Save(ctx, p)
}
