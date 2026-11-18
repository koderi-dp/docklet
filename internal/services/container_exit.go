package services

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/moby/moby/client"

	"docklet/internal/compose"
	"docklet/internal/model"
)

var exitedStatusRe = regexp.MustCompile(`^Exited \((-?\d+)\)`)

// parseExitCode reads N from the Engine status text "Exited (N) 3 hours ago".
// The list API has no structured exit code; nil means not exited or unparsable.
func parseExitCode(status string) *int {
	m := exitedStatusRe.FindStringSubmatch(strings.TrimSpace(status))
	if m == nil {
		return nil
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return nil
	}
	return &n
}

// policyCache remembers the restart policy of containers that exited with code 0, keyed
// by connection, ID and creation time so a recreated container is looked up again. It
// keeps the list call free of one inspect per container.
type policyCache struct {
	mu sync.Mutex
	m  map[string]string
}

var exitedPolicies policyCache

const policyCacheMax = 4096

func policyKey(connectionID, id string, created int64) string {
	return connectionID + "|" + id + "|" + strconv.FormatInt(created, 10)
}

func (c *policyCache) get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[key]
	return v, ok
}

func (c *policyCache) put(key, v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil || len(c.m) >= policyCacheMax {
		c.m = map[string]string{}
	}
	c.m[key] = v
}

// policyTarget is an exited-with-0 container whose restart policy is still unknown.
type policyTarget struct {
	id, key string
	set     func(policy string)
}

// resolveRestartPolicies fills the restart policy of the targets: from the cache, else with
// one bounded inspect per container (only ever for exit code 0, usually a handful). A failed
// inspect leaves the policy empty (unknown), which never counts as completed.
func resolveRestartPolicies(ctx context.Context, cli *client.Client, targets []policyTarget) {
	var todo []policyTarget
	for _, t := range targets {
		if v, ok := exitedPolicies.get(t.key); ok {
			t.set(v)
		} else {
			todo = append(todo, t)
		}
	}
	forEachBounded(ctx, len(todo), func(i int) {
		t := todo[i]
		res, err := cli.ContainerInspect(ctx, t.id, client.ContainerInspectOptions{})
		if err != nil || res.Container.HostConfig == nil {
			return
		}
		p := string(res.Container.HostConfig.RestartPolicy.Name)
		if p == "" {
			p = "no"
		}
		exitedPolicies.put(t.key, p)
		t.set(p)
	}, nil)
}

// classifyExited sets ExitCode and RestartPolicy on list rows. Compose one-offs are
// "no" by definition; other exit-0 containers need a (cached) inspect.
func classifyExited(ctx context.Context, cli *client.Client, connectionID string, rows []model.Container, oneOff func(i int) bool) {
	var targets []policyTarget
	for i := range rows {
		r := &rows[i]
		if r.State != "exited" {
			continue
		}
		r.ExitCode = parseExitCode(r.Status)
		if r.ExitCode == nil || *r.ExitCode != 0 {
			continue
		}
		if oneOff(i) {
			r.RestartPolicy = "no"
			continue
		}
		targets = append(targets, policyTarget{id: r.ID, key: policyKey(connectionID, r.ID, r.Created),
			set: func(p string) { r.RestartPolicy = p }})
	}
	resolveRestartPolicies(ctx, cli, targets)
}

func isOneOffLabels(l map[string]string) bool {
	return strings.EqualFold(l[compose.LabelOneOff], "true")
}
