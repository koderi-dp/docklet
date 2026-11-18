package services

import (
	"strings"

	"docklet/internal/buildkit"
	"docklet/internal/model"
)

// classicSink forwards an Engine API build's output to the operation and remembers what the history needs.
type classicSink struct {
	h       pullSink
	tail    tailBuffer
	total   int
	cached  int
	imageID string
}

func (c *classicSink) Log(l string) {
	c.tail.Add(l)
	if sm := stepRe.FindStringSubmatch(l); sm != nil {
		c.total = atoiSafe(sm[2])
	}
	if strings.Contains(l, "Using cache") {
		c.cached++
	}
	if rest, ok := strings.CutPrefix(l, "Built image "); ok {
		c.imageID = strings.TrimSpace(rest)
	}
	c.h.Log(l)
}
func (c *classicSink) Detail(s string)    { c.h.Detail(s) }
func (c *classicSink) Progress(p float64) { c.h.Progress(p) }

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// classicMeta describes a Classic (Engine API) build for the history.
func classicMeta(connectionID string, spec model.BuildSpec, v validatedBuild) buildMeta {
	names := sortedArgNames(spec.BuildArgs)
	var args []buildkit.Arg
	for _, k := range names {
		args = append(args, buildkit.Arg{Name: strings.TrimSpace(k), Value: spec.BuildArgs[k]})
	}
	opts := buildkit.Options{Method: buildkit.MethodClassic, ContextDir: v.ContextDir, Dockerfile: joinPath(v.ContextDir, v.Dockerfile),
		Tags: v.Tags, Args: args, Target: v.Target, Platform: strings.TrimSpace(spec.Platform), NoCache: v.NoCache, Pull: v.Pull}
	m := buildMeta{connectionID: connectionID, builder: model.BuilderClassic, method: "Engine API", contextDir: v.ContextDir,
		dockerfile: v.Dockerfile, tags: v.Tags, target: v.Target, platform: opts.Platform, noCache: v.NoCache, pull: v.Pull,
		command: buildkit.CopyCommand(opts), started: timeNow(), hide: buildkit.NewScrubber(buildkit.AllValues(args), 4)}
	m.argNames = append(m.argNames, names...)
	return m
}
