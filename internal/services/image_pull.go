package services

import (
	"context"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/client"

	"docklet/internal/model"
	"docklet/internal/ops"
)

// layerState tracks one layer of a pull.
type layerState struct {
	status  string
	current int64
	total   int64
	done    bool
}

// pullTracker aggregates the Engine's pull JSON stream. Progress is only
// reported when every layer that needs downloading has a known byte total, so
// the figure is real and never jumps backwards when new layers appear.
type pullTracker struct {
	layers map[string]*layerState
}

func newPullTracker() *pullTracker { return &pullTracker{layers: map[string]*layerState{}} }

// Observe records a message and returns a log line when the layer's status
// changed (progress ticks within the same status are not logged).
func (p *pullTracker) Observe(m jsonstream.Message) string {
	if !isLayerID(m.ID) {
		// Whole-image lines such as "Digest: sha256:..." / "Status: Downloaded newer image".
		if m.Status != "" {
			return strings.TrimSpace(m.Status + " " + m.ID)
		}
		return ""
	}
	l, ok := p.layers[m.ID]
	if !ok {
		l = &layerState{}
		p.layers[m.ID] = l
	}
	statusChanged := m.Status != "" && m.Status != l.status
	if m.Status != "" {
		l.status = m.Status
	}
	switch {
	case strings.HasPrefix(m.Status, "Downloading"):
		if m.Progress != nil && m.Progress.Total > 0 {
			l.current, l.total = m.Progress.Current, m.Progress.Total
		}
	case m.Status == "Download complete", m.Status == "Pull complete", m.Status == "Already exists",
		m.Status == "Verifying Checksum", strings.HasPrefix(m.Status, "Extracting"):
		if l.total > 0 {
			l.current = l.total
		}
		if m.Status == "Pull complete" || m.Status == "Already exists" {
			l.done = true
		}
	}
	if statusChanged {
		return m.ID + ": " + m.Status
	}
	return ""
}

// Fraction returns downloaded/total bytes across layers and whether it is known.
func (p *pullTracker) Fraction() (float64, bool) {
	var cur, tot int64
	for _, l := range p.layers {
		if l.status == "Already exists" {
			continue
		}
		if l.total <= 0 {
			if l.done {
				continue
			}
			return 0, false // a layer's size is not known yet
		}
		tot += l.total
		cur += l.current
	}
	if tot <= 0 {
		return 0, false
	}
	f := float64(cur) / float64(tot)
	if f > 1 {
		f = 1
	}
	return f, true
}

// Bytes returns aggregate downloaded and total bytes of layers with known totals.
func (p *pullTracker) Bytes() (cur, tot int64) {
	for _, l := range p.layers {
		if l.total > 0 && l.status != "Already exists" {
			cur += l.current
			tot += l.total
		}
	}
	return
}

func isLayerID(id string) bool {
	if len(id) < 6 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Pull starts a background pull and returns the operation immediately. The
// reference is validated synchronously. Docker CLI credentials for the
// registry (config.json / credential helpers) are used when present.
func (s *ImageService) Pull(ctx context.Context, connectionID, ref string) (model.Operation, error) {
	ref, err := validateRepoTag(ref)
	if err != nil {
		return model.Operation{}, err
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.Operation{}, err
	}
	if s.Ops == nil {
		return model.Operation{}, fmt.Errorf("operations are not available")
	}
	op := s.Ops.Start(ops.Spec{
		Kind: "image.pull", Title: "Pull " + ref, ConnectionID: connectionID, Resource: ref, Cancellable: true,
	}, func(octx context.Context, h *ops.Handle) error {
		err := s.runPull(octx, cli, ref, h)
		s.emitChanged(connectionID) // even on failure: a partial pull may have changed layers
		return err
	})
	return op, nil
}

func (s *ImageService) runPull(ctx context.Context, cli *client.Client, ref string, h *ops.Handle) error {
	h.Log("Pulling " + ref)
	creds := s.authFor(ctx, ref, h)
	resp, err := cli.ImagePull(ctx, ref, client.ImagePullOptions{RegistryAuth: creds.HeaderValue()})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return imageError(registryAuthError(err, creds, ref, true))
	}
	defer resp.Close()
	return registryAuthError(consumePull(ctx, resp.JSONMessages(ctx), h), creds, ref, true)
}

type pullSink interface {
	Log(string)
	Detail(string)
	Progress(float64)
}

func consumePull(ctx context.Context, msgs iter.Seq2[jsonstream.Message, error], h pullSink) error {
	tr := newPullTracker()
	var lastDetail time.Time
	for m, err := range msgs {
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return imageError(err)
		}
		if m.Error != nil {
			return imageError(fmt.Errorf("%s", m.Error.Message))
		}
		if line := tr.Observe(m); line != "" {
			h.Log(line)
		}
		if f, ok := tr.Fraction(); ok {
			h.Progress(f)
			if time.Since(lastDetail) > 200*time.Millisecond {
				lastDetail = time.Now()
				cur, tot := tr.Bytes()
				h.Detail(fmt.Sprintf("Downloading %s / %s (%d layers)", humanBytes(cur), humanBytes(tot), len(tr.layers)))
			}
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
