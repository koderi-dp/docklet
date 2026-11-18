package runspec

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/moby/moby/api/types/container"
)

// PasswordPlaceholder stands in for the password unless the caller explicitly asks for it.
const PasswordPlaceholder = "<password>"

// ConnContext describes how this machine reaches the Engine (see ConnectHost).
type ConnContext struct {
	Type     string // npipe, unix, wsl, tcp, ssh or context
	Endpoint string
}

// ConnEntry is one copyable line. UsesPassword marks entries that contain the password (or its placeholder).
type ConnEntry struct {
	Label        string
	Text         string
	UsesPassword bool
	// Internal entries use the name other containers resolve, not an address of this computer.
	Internal bool
}

// ConnInfo is the result of ConnectionStrings.
type ConnInfo struct {
	Kind, Title string
	Host        string // address of the first entry
	Reachable   bool   // true when the entries use an address this machine can reach
	Note        string // why the entries are the internal ones, or other caveats
	// PasswordEnv names the container env variable that holds the password ("" when none was set).
	PasswordEnv string
	Entries     []ConnEntry
}

// ConnInput is what ConnectionStrings reads from an inspected container.
type ConnInput struct {
	Image, Name, Service string
	Env                  map[string]string
	Exposed              map[int]bool
	// Published are the actual host bindings (running) or the requested ones (stopped).
	Published []Port
	Running   bool
}

// ConnInputFromInspect extracts a ConnInput.
func ConnInputFromInspect(in container.InspectResponse) ConnInput {
	ci := ConnInput{Name: strings.TrimPrefix(in.Name, "/"), Env: map[string]string{}, Exposed: map[int]bool{}}
	ci.Running = in.State != nil && in.State.Running
	if c := in.Config; c != nil {
		ci.Image, ci.Service = c.Image, c.Labels["com.docker.compose.service"]
		for _, e := range c.Env {
			k, v, _ := strings.Cut(e, "=")
			ci.Env[k] = v
		}
		for p := range c.ExposedPorts {
			if p.Proto() == "tcp" {
				ci.Exposed[int(p.Num())] = true
			}
		}
	}
	if ns := in.NetworkSettings; ns != nil && ci.Running {
		for port, bs := range ns.Ports {
			for _, b := range bs {
				p := Port{HostPort: b.HostPort, ContainerPort: int(port.Num()), Proto: string(port.Proto())}
				if b.HostIP.IsValid() {
					p.HostIP = b.HostIP.String()
				}
				ci.Published = append(ci.Published, p)
			}
		}
	} else if in.HostConfig != nil {
		ci.Published = mapPorts(in.HostConfig)
	}
	return ci
}

type dbKind struct {
	id, title string
	names     []string // substrings of the lower-cased repository
	ports     []int
	creds     func(b *connBuilder)
	entries   func(b *connBuilder, containerPort int)
}

var notDatabase = []string{"exporter", "express", "commander", "insight", "admin", "bouncer", "operator", "backup", "proxy", "sentinel"}

var digestOrTag = regexp.MustCompile(`(@.*|:[^/]*)$`)

func imageRepo(ref string) string { return strings.ToLower(digestOrTag.ReplaceAllString(ref, "")) }

// DetectDatabase returns the database kind id of a container ("" when none is recognised).
func DetectDatabase(in ConnInput) string {
	k := detect(in)
	if k == nil {
		return ""
	}
	return k.id
}

func detect(in ConnInput) *dbKind {
	repo := imageRepo(in.Image)
	for _, bad := range notDatabase {
		if strings.Contains(repo, bad) {
			return nil
		}
	}
	for i := range dbKinds {
		k := &dbKinds[i]
		for _, n := range k.names {
			if !strings.Contains(repo, n) {
				continue
			}
			for _, p := range k.ports {
				if in.Exposed[p] {
					return k
				}
			}
		}
	}
	return nil
}

// ConnectionStrings builds the connection strings for a recognised database container; ok is false
// when the container is not a known database. With includePassword the real password replaces the placeholder.
func ConnectionStrings(in ConnInput, ctx ConnContext, includePassword bool) (ConnInfo, bool) {
	k := detect(in)
	if k == nil {
		return ConnInfo{}, false
	}
	b := &connBuilder{in: in, includePassword: includePassword}
	info := ConnInfo{Kind: k.id, Title: k.title}
	k.creds(b)
	info.PasswordEnv = b.passVar
	internalName := in.Service
	if internalName == "" {
		internalName = in.Name
	}
	for _, p := range k.ports {
		if !in.Exposed[p] {
			continue
		}
		ok, host, note := b.resolveHost(ctx, []int{p})
		b.host, b.port, b.internal = host, b.hostPort(p), !ok
		if !ok {
			b.host, b.port = internalName, p
			if !strings.Contains(info.Note, note) {
				info.Note = strings.TrimSpace(info.Note + " " + note)
			}
		} else {
			info.Reachable = true
		}
		if info.Host == "" {
			info.Host = b.host
		}
		b.entries = nil
		k.entries(b, p)
		info.Entries = append(info.Entries, b.entries...)
	}
	return info, len(info.Entries) > 0
}

// connBuilder carries the resolved credentials and collects entries while a kind builds them.
type connBuilder struct {
	in              ConnInput
	includePassword bool
	host            string
	port            int
	entries         []ConnEntry
	internal        bool
	// credentials found in the env
	user, db, passVar string
	passValue         string
	alwaysPassword    bool
	vhost             string
}

func (b *connBuilder) env(keys ...string) (string, string) {
	for _, k := range keys {
		if v, ok := b.in.Env[k]; ok && v != "" {
			return k, v
		}
	}
	return "", ""
}

func (b *connBuilder) setPassword(keys ...string) { b.passVar, b.passValue = b.env(keys...) }

// password is the text that goes into a URL: the placeholder, or the real value when allowed.
func (b *connBuilder) password() (text string, real bool) {
	if b.includePassword && b.passValue != "" {
		return url.UserPassword("", b.passValue).String()[1:], true
	}
	return PasswordPlaceholder, false
}

func (b *connBuilder) hasPassword() bool { return b.passVar != "" || b.alwaysPassword }

// userinfo renders user[:password]@ (empty when there is nothing to put).
func (b *connBuilder) userinfo() string {
	u := url.User(b.user).String()
	if b.user == "" && !b.hasPassword() {
		return ""
	}
	if !b.hasPassword() {
		return u + "@"
	}
	p, _ := b.password()
	return u + ":" + p + "@"
}

func (b *connBuilder) hostPort(containerPort int) int {
	for _, p := range b.in.Published {
		if p.ContainerPort == containerPort && p.Proto == "tcp" {
			n, _ := strconv.Atoi(p.HostPort)
			return n
		}
	}
	return 0
}

func (b *connBuilder) addr() string {
	h := b.host
	if strings.Contains(h, ":") && !strings.HasPrefix(h, "[") {
		h = "[" + h + "]"
	}
	return h + ":" + strconv.Itoa(b.port)
}

func (b *connBuilder) add(label, text string, usesPassword bool) {
	b.entries = append(b.entries, ConnEntry{Label: label, Text: text, UsesPassword: usesPassword && b.hasPassword(), Internal: b.internal})
}

func wildcardHost(h string) bool { return h == "" || h == "0.0.0.0" || h == "::" || h == "[::]" }
func loopbackHost(h string) bool {
	return h == "127.0.0.1" || h == "::1" || h == "[::1]" || h == "localhost"
}

// resolveHost picks the address a program on this computer uses for the first published port of the kind.
func (b *connBuilder) resolveHost(ctx ConnContext, ports []int) (reachable bool, host, note string) {
	internal := b.in.Service
	if internal == "" {
		internal = b.in.Name
	}
	var pub *Port
	for _, cp := range ports {
		if !b.in.Exposed[cp] {
			continue
		}
		for i, p := range b.in.Published {
			if p.ContainerPort == cp && p.Proto == "tcp" && p.HostPort != "" && p.HostPort != "0" {
				pub = &b.in.Published[i]
				break
			}
		}
		if pub != nil {
			break
		}
	}
	notPublished := fmt.Sprintf("Not published: reachable only from other containers as %s:%d.", internal, firstExposed(ports, b.in.Exposed))
	if pub == nil {
		if b.hasDynamicPort(ports) && !b.in.Running {
			return false, "", "The host port is assigned when the container starts. " + notPublished
		}
		return false, "", notPublished
	}
	host, reason := ConnectHost(ctx, pub.HostIP)
	if reason != "" {
		return false, "", reason + " Other containers can use " + internal + "."
	}
	return true, host, ""
}

func (b *connBuilder) hasDynamicPort(ports []int) bool {
	for _, p := range b.in.Published {
		for _, cp := range ports {
			if p.ContainerPort == cp && (p.HostPort == "" || p.HostPort == "0") {
				return true
			}
		}
	}
	return false
}

func firstExposed(ports []int, exposed map[int]bool) int {
	for _, p := range ports {
		if exposed[p] {
			return p
		}
	}
	return ports[0]
}

// ConnectHost mirrors the frontend's portUrl logic: the host a program on this computer uses to reach a
// published port, or a reason why it cannot be determined.
func ConnectHost(ctx ConnContext, bindIP string) (host, reason string) {
	bind := strings.ToLower(strings.TrimSpace(bindIP))
	wildcard, loopback := wildcardHost(bind), loopbackHost(bind)
	typ := ctx.Type
	if typ == "context" {
		typ = endpointScheme(ctx.Endpoint)
	}
	switch typ {
	case "npipe", "unix", "wsl":
		if !wildcard && !loopback {
			return "", fmt.Sprintf("Bound to %s, which may not be reachable as localhost.", bindIP)
		}
		return "localhost", ""
	case "tcp", "ssh":
		h := EndpointHost(ctx.Endpoint)
		switch {
		case h == "":
			return "", "The engine host could not be read from the connection endpoint."
		case !wildcard && loopback:
			return "", fmt.Sprintf("Bound to %s on the engine host, so it is not reachable from this machine.", bindIP)
		case !wildcard:
			return "", fmt.Sprintf("Bound to %s; the host is not guessed.", bindIP)
		}
		return h, ""
	}
	return "", "The host of this connection type cannot be determined."
}

var schemeRe = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9+.-]*)://`)

func endpointScheme(e string) string {
	if m := schemeRe.FindStringSubmatch(strings.TrimSpace(e)); m != nil {
		return strings.ToLower(m[1])
	}
	return ""
}

// EndpointHost returns the host of tcp://host:2376 or ssh://user@host:22 ("" when absent).
func EndpointHost(endpoint string) string {
	rest := schemeRe.ReplaceAllString(strings.TrimSpace(endpoint), "")
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		rest = rest[:i]
	}
	if i := strings.LastIndex(rest, "@"); i >= 0 {
		rest = rest[i+1:]
	}
	if strings.HasPrefix(rest, "[") {
		if end := strings.Index(rest, "]"); end > 0 {
			return rest[1:end]
		}
		return ""
	}
	if i := strings.LastIndex(rest, ":"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}
