package registryauth

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
)

// Where an entry is kept.
const (
	KindHelper  = "helper"  // a docker-credential-<name> helper (the docker CLI shares it)
	KindDocklet = "docklet" // Docklet's own entry in the OS credential store
	KindConfig  = "config"  // plain data in config.json (read-only legacy; sign out removes it)
)

// Entry is a registry sign-in without its secret.
type Entry struct {
	Host     string
	Username string // "<token>" for identity tokens
	Kind     string
	Helper   string // helper name when Kind is KindHelper
	Label    string
}

// Label describes where credentials live, for the UI.
func Label(kind, helper string) string {
	switch kind {
	case KindHelper:
		return helperLabel(helper)
	case KindConfig:
		return "Docker config file (plain text)"
	}
	return "Docklet (" + osStoreName() + ")"
}

func osStoreName() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows Credential Manager"
	case "darwin":
		return "macOS Keychain"
	}
	return "Secret Service"
}

func helperLabel(name string) string {
	bin := "docker-credential-" + name
	switch name {
	case "wincred":
		return "Windows Credential Manager via " + bin
	case "osxkeychain":
		return "macOS Keychain via " + bin
	case "secretservice":
		return "Secret Service via " + bin
	case "desktop":
		return "Docker Desktop credential store via " + bin
	case "pass":
		return "pass via " + bin
	}
	return "Credential helper " + bin
}

// target decides where a new sign-in for host is stored: the credential helper
// the Docker CLI would use when it is installed, otherwise Docklet's own entry.
func (r *Resolver) target(host string) (kind, helper string) {
	cfg, err := r.loadConfig()
	if err != nil {
		return KindDocklet, ""
	}
	if h := cfg.helperFor(host); h != "" && helperOnPath(r.LookPath, h) {
		return KindHelper, h
	}
	return KindDocklet, ""
}

// StorageLabel says where a sign-in for host would be stored.
func (r *Resolver) StorageLabel(host string) string {
	k, h := r.target(NormalizeHost(host))
	return Label(k, h)
}

// Save stores credentials (an identity token as username "<token>"). Errors
// never contain the secret.
func (r *Resolver) Save(ctx context.Context, host, username, secret string) (Entry, error) {
	host = NormalizeHost(host)
	if host == "" || username == "" || secret == "" {
		return Entry{}, errors.New("registry, user name and password are required")
	}
	kind, helper := r.target(host)
	e := Entry{Host: host, Username: username, Kind: kind, Helper: helper, Label: Label(kind, helper)}
	switch kind {
	case KindHelper:
		if err := (Helper{Name: helper, Runner: r.Runner}).Store(ctx, ServerKey(host), username, secret); err != nil {
			return Entry{}, err
		}
		_ = markHelperEntry(r.configPath(), ServerKey(host)) // the docker CLI keeps this empty marker too
	default:
		if err := SaveDocklet(r.Store, host, username, secret); err != nil {
			return Entry{}, err
		}
	}
	return e, nil
}

// Remove deletes the sign-in of host from the place the entry lives.
func (r *Resolver) Remove(ctx context.Context, host, kind string) error {
	host = NormalizeHost(host)
	switch kind {
	case KindHelper:
		cfg, err := r.loadConfig()
		if err != nil {
			return err
		}
		name := cfg.helperFor(host)
		if name == "" {
			return fmt.Errorf("no credential helper is configured for %s", host)
		}
		if err := (Helper{Name: name, Runner: r.Runner}).Erase(ctx, ServerKey(host)); err != nil {
			return err
		}
		return removeAuthsEntry(r.configPath(), host, true)
	case KindDocklet:
		return DeleteDocklet(r.Store, host)
	case KindConfig:
		return removeAuthsEntry(r.configPath(), host, false)
	}
	return fmt.Errorf("unknown credential location %q", kind)
}

// List returns every sign-in Docklet can use, one per registry (helper, then
// Docklet, then config.json). Warnings describe sources that could not be read.
func (r *Resolver) List(ctx context.Context) ([]Entry, []string) {
	var warns []string
	seen := map[string]bool{}
	var out []Entry
	add := func(e Entry) {
		if !seen[e.Host] {
			seen[e.Host] = true
			e.Label = Label(e.Kind, e.Helper)
			out = append(out, e)
		}
	}
	cfg, err := r.loadConfig()
	if err != nil {
		warns = append(warns, err.Error())
	}
	listed := map[string]bool{}
	if cfg.CredsStore != "" {
		listed[cfg.CredsStore] = true
		if !helperOnPath(r.LookPath, cfg.CredsStore) {
			warns = append(warns, fmt.Sprintf("docker-credential-%s was not found on PATH, so its saved sign-ins cannot be listed", cfg.CredsStore))
		} else if m, err := (Helper{Name: cfg.CredsStore, Runner: r.Runner}).List(ctx); err != nil {
			warns = append(warns, err.Error())
		} else {
			for url, user := range m {
				if isRegistryURL(url) {
					add(Entry{Host: NormalizeHost(url), Username: user, Kind: KindHelper, Helper: cfg.CredsStore})
				}
			}
		}
	}
	for host, name := range cfg.CredHelpers {
		if listed[name] || !helperOnPath(r.LookPath, name) {
			continue
		}
		if user, secret, err := (Helper{Name: name, Runner: r.Runner}).Get(ctx, ServerKey(NormalizeHost(host))); err == nil && secret != "" {
			add(Entry{Host: NormalizeHost(host), Username: user, Kind: KindHelper, Helper: name})
		}
	}
	if docklet, err := ListDocklet(r.Store); err != nil {
		warns = append(warns, err.Error())
	} else {
		for _, d := range docklet {
			add(Entry{Host: d.Host, Username: d.Username, Kind: KindDocklet})
		}
	}
	for k, a := range cfg.Auths {
		if c := entryToCredentials(NormalizeHost(k), a); c.Has() {
			user := c.Username
			if c.IdentityToken != "" {
				user = "<token>"
			}
			add(Entry{Host: NormalizeHost(k), Username: user, Kind: KindConfig})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out, warns
}

// isRegistryURL tells registry entries from other items a helper may hold
// (Docker Desktop keeps tokens under https://index.docker.io/v1/access-token).
func isRegistryURL(u string) bool {
	s := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(u)), "https://"), "http://")
	i := strings.Index(s, "/")
	if i < 0 {
		return true
	}
	rest := strings.Trim(s[i:], "/")
	return rest == "" || rest == "v1"
}
