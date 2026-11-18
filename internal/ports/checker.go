package ports

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// EngineSource lists the published ports of every container on one Engine.
type EngineSource interface {
	Bindings(ctx context.Context) ([]Binding, error)
}

// HostSource lists listening sockets on one machine (Windows, a WSL distribution, ...).
type HostSource interface {
	Where() string
	Listeners(ctx context.Context) ([]Listener, error)
}

// Checker combines the Engine's view with the host's. HostUnknown is set (to the
// reason) when the Engine runs on another computer, so no host source applies.
type Checker struct {
	Engine      EngineSource
	Hosts       []HostSource
	HostUnknown string
	Timeout     time.Duration // whole call; default 2s
}

// snapshot is everything read once per call; evaluating each request is pure.
type snapshot struct {
	bindings  []Binding
	problems  []string // sources that could not be read: what they might hold is "unknown", never "free"
	listeners []Listener
	hostGap   string // why the host side is not covered at all
}

func (c *Checker) snap(ctx context.Context) (snapshot, context.CancelFunc) {
	d := c.Timeout
	if d <= 0 {
		d = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, d)
	s := snapshot{hostGap: c.HostUnknown}
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1 + len(c.Hosts))
	go func() {
		defer wg.Done()
		b, err := c.Engine.Bindings(ctx)
		mu.Lock()
		defer mu.Unlock()
		s.bindings = b
		if err != nil {
			s.problems = append(s.problems, "the containers of this Engine could not be read ("+short(err)+")")
		}
	}()
	for _, h := range c.Hosts {
		go func() {
			defer wg.Done()
			ls, err := h.Listeners(ctx)
			mu.Lock()
			defer mu.Unlock()
			s.listeners = append(s.listeners, ls...)
			if err != nil {
				s.problems = append(s.problems, "the listeners on "+h.Where()+" could not be read ("+short(err)+")")
			}
		}()
	}
	wg.Wait()
	return s, cancel
}

func short(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timed out"
	}
	m := strings.TrimSpace(err.Error())
	if len(m) > 160 {
		m = m[:160] + "..."
	}
	return m
}

// Check answers each request. ignore lists containers (being recreated, or part of the
// Compose project being started) whose own bindings are not conflicts.
func (c *Checker) Check(ctx context.Context, reqs []Request, ignore []string) []PortCheck {
	s, cancel := c.snap(ctx)
	defer cancel()
	skip := toSet(ignore)
	out := make([]PortCheck, len(reqs))
	for i, r := range reqs {
		out[i] = s.evaluate(r, skip)
	}
	return out
}

// List returns every published port and every host listener that is not just a proxy
// for a published port.
func (c *Checker) List(ctx context.Context) Listing {
	s, cancel := c.snap(ctx)
	defer cancel()
	return s.listing()
}

func toSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func (s snapshot) evaluate(r Request, skip map[string]bool) PortCheck {
	proto := normProto(r.Protocol)
	res := PortCheck{HostIP: r.HostIP, HostPort: r.HostPort, Protocol: proto, Also: []Holder{}}
	var holders []Holder
	seen := map[string]bool{}
	add := func(h Holder, key string) {
		if !seen[key] {
			seen[key] = true
			holders = append(holders, h)
		}
	}
	for _, b := range s.bindings {
		if b.HostPort != r.HostPort || normProto(b.Protocol) != proto || !overlap(b.HostIP, r.HostIP) || skip[b.ContainerID] {
			continue
		}
		if b.Running {
			add(containerHolder(b), "c:"+b.ContainerID)
		} else {
			res.Note = fmt.Sprintf("The stopped container %s also publishes this port; it cannot start while the port is in use.", b.Name)
		}
	}
	for _, l := range s.listeners {
		if l.Port != r.HostPort || normProto(l.Protocol) != proto || !overlap(l.IP, r.HostIP) {
			continue
		}
		if isProxy(l.Name) || l.anonymous() {
			if owners := s.owners(l); len(owners) > 0 {
				for _, b := range owners {
					if !skip[b.ContainerID] {
						add(containerHolder(b), "c:"+b.ContainerID)
					}
				}
				continue // the proxy belongs to a container (own or conflicting)
			}
		}
		add(listenerHolder(l), fmt.Sprintf("p:%s:%d", l.Location, l.PID))
	}
	sortHolders(holders)
	if len(holders) > 0 {
		res.Status = StatusInUse
		res.Holder = &holders[0]
		res.Also = append(res.Also, holders[1:]...)
		res.Detail = describePort(r.HostPort, proto) + " is used by " + describe(holders[0])
		if len(holders) > 1 {
			res.Detail += fmt.Sprintf(" (and %d more)", len(holders)-1)
		}
		return res
	}
	if reason := s.unknownReason(proto); reason != "" {
		res.Status, res.Detail = StatusUnknown, "Could not check: "+reason
		return res
	}
	res.Status = StatusFree
	return res
}

// unknownReason is why "nothing found" cannot be read as "free", or "".
func (s snapshot) unknownReason(proto string) string {
	why := append([]string{}, s.problems...)
	if s.hostGap != "" {
		why = append(why, s.hostGap)
	} else if proto != "tcp" && proto != "udp" {
		why = append(why, "listeners for "+proto+" cannot be read on this machine")
	}
	return strings.Join(why, "; ")
}

// owners are the running containers whose published port a proxy listener serves.
func (s snapshot) owners(l Listener) []Binding {
	var out []Binding
	for _, b := range s.bindings {
		if b.Running && b.HostPort == l.Port && normProto(b.Protocol) == normProto(l.Protocol) && overlap(b.HostIP, l.IP) {
			out = append(out, b)
		}
	}
	return out
}

func (s snapshot) listing() Listing {
	out := Listing{Entries: []Entry{}, Notes: []string{}}
	for _, b := range s.bindings {
		if b.HostPort > 0 {
			out.Entries = append(out.Entries, Entry{HostIP: b.HostIP, HostPort: b.HostPort, Protocol: normProto(b.Protocol), Holder: containerHolder(b)})
		}
	}
	seen := map[string]bool{}
	hidden := false
	for _, l := range s.listeners {
		if (isProxy(l.Name) || l.anonymous()) && len(s.owners(l)) > 0 {
			continue
		}
		key := fmt.Sprintf("%s|%s|%d|%s|%d", l.Location, l.IP, l.Port, l.Protocol, l.PID)
		if seen[key] {
			continue
		}
		seen[key] = true
		hidden = hidden || l.Hidden || l.Name == ""
		out.Entries = append(out.Entries, Entry{HostIP: l.IP, HostPort: l.Port, Protocol: normProto(l.Protocol), Holder: listenerHolder(l)})
	}
	sort.SliceStable(out.Entries, func(i, j int) bool {
		a, b := out.Entries[i], out.Entries[j]
		if a.HostPort != b.HostPort {
			return a.HostPort < b.HostPort
		}
		return a.Protocol < b.Protocol
	})
	for _, p := range s.problems {
		out.Notes = append(out.Notes, "Not shown: "+p+".")
	}
	if s.hostGap != "" {
		out.Notes = append(out.Notes, s.hostGap+" Only the ports published by containers are shown.")
	}
	if hidden {
		out.Notes = append(out.Notes, "Some programs belong to other users or to the system, so their name, path or owner may be hidden.")
	}
	return out
}

func containerHolder(b Binding) Holder {
	loc := b.Engine
	if loc == "" {
		loc = "Docker Engine"
	}
	return Holder{Kind: HolderContainer, Location: loc, BindIP: b.HostIP, ContainerID: b.ContainerID,
		ContainerName: b.Name, ComposeProject: b.Project, Running: b.Running}
}

func listenerHolder(l Listener) Holder {
	k := HolderProcess
	if l.System {
		k = HolderSystem
	}
	return Holder{Kind: k, Location: l.Location, BindIP: l.IP, PID: l.PID, ProcessName: l.Name, ExecutablePath: l.Path, User: l.User}
}

// sortHolders puts containers first (they are what Docker itself trips over).
func sortHolders(h []Holder) {
	sort.SliceStable(h, func(i, j int) bool { return h[i].Kind == HolderContainer && h[j].Kind != HolderContainer })
}

func describePort(port int, proto string) string {
	if proto == "tcp" {
		return fmt.Sprint(port)
	}
	return fmt.Sprintf("%d/%s", port, proto)
}

func describe(h Holder) string {
	if h.Kind == HolderContainer {
		s := "container " + h.ContainerName
		if h.ComposeProject != "" {
			s += " (project " + h.ComposeProject + ")"
		}
		return s
	}
	name := h.ProcessName
	if name == "" {
		name = "a program that could not be identified"
	}
	if h.PID > 0 {
		name += fmt.Sprintf(" (PID %d)", h.PID)
	}
	if h.Kind == HolderSystem {
		name = "the system: " + name
	}
	if h.Location != "" && h.Location != "This machine" {
		name += " on " + h.Location
	}
	return name
}
