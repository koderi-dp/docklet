package services

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/client"

	"docklet/internal/model"
	"docklet/internal/ops"
	"docklet/internal/registryauth"
)

// pushTracker aggregates the Engine's push JSON stream. Progress is only
// reported while every layer that is being uploaded has a known byte total.
type pushTracker struct {
	layers map[string]*layerState
}

func newPushTracker() *pushTracker { return &pushTracker{layers: map[string]*layerState{}} }

// Observe records a message and returns a log line for status changes.
func (p *pushTracker) Observe(m jsonstream.Message) string {
	if !isLayerID(m.ID) {
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
	changed := m.Status != "" && m.Status != l.status
	if m.Status != "" {
		l.status = m.Status
	}
	switch {
	case m.Status == "Pushing":
		if m.Progress != nil && m.Progress.Total > 0 {
			l.current, l.total = m.Progress.Current, m.Progress.Total
		}
	case m.Status == "Pushed":
		if l.total > 0 {
			l.current = l.total
		}
		l.done = true
	case m.Status == "Layer already exists", strings.HasPrefix(m.Status, "Mounted from"):
		l.done = true
	}
	if changed {
		return m.ID + ": " + m.Status
	}
	return ""
}

// Fraction returns pushed/total bytes over the layers that upload, if known.
func (p *pushTracker) Fraction() (float64, bool) {
	var cur, tot int64
	for _, l := range p.layers {
		if l.status == "Layer already exists" || strings.HasPrefix(l.status, "Mounted from") {
			continue
		}
		if l.total <= 0 {
			if l.done {
				continue
			}
			return 0, false
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

func (p *pushTracker) Bytes() (cur, tot int64) {
	for _, l := range p.layers {
		if l.total > 0 {
			cur += l.current
			tot += l.total
		}
	}
	return
}

func consumePush(ctx context.Context, msgs iter.Seq2[jsonstream.Message, error], h pullSink) error {
	tr := newPushTracker()
	var lastDetail time.Time
	for m, err := range msgs {
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return imageError(err)
		}
		if m.Error != nil {
			return imageError(errors.New(m.Error.Message))
		}
		if line := tr.Observe(m); line != "" {
			h.Log(line)
		}
		if f, ok := tr.Fraction(); ok {
			h.Progress(f)
			if time.Since(lastDetail) > 200*time.Millisecond {
				lastDetail = time.Now()
				cur, tot := tr.Bytes()
				h.Detail(fmt.Sprintf("Uploading %s / %s (%d layers)", humanBytes(cur), humanBytes(tot), len(tr.layers)))
			}
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

// Push uploads a local image to its registry as a background operation.
// Credentials come from the Docker CLI configuration on this computer.
func (s *ImageService) Push(ctx context.Context, connectionID, ref string) (model.Operation, error) {
	ref, err := validateRepoTag(ref)
	if err != nil {
		return model.Operation{}, err
	}
	if n, _ := reference.ParseNormalizedNamed(ref); n != nil {
		if _, ok := n.(reference.Digested); ok {
			return model.Operation{}, errors.New("a digest reference cannot be pushed; push a repository:tag")
		}
	}
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.Operation{}, err
	}
	if s.Ops == nil {
		return model.Operation{}, fmt.Errorf("operations are not available")
	}
	op := s.Ops.Start(ops.Spec{
		Kind: "image.push", Title: "Push " + ref, ConnectionID: connectionID, Resource: ref, Cancellable: true,
	}, func(octx context.Context, h *ops.Handle) error {
		return s.runPush(octx, cli, ref, h)
	})
	return op, nil
}

func (s *ImageService) runPush(ctx context.Context, cli *client.Client, ref string, h *ops.Handle) error {
	h.Log("Pushing " + ref)
	creds, rerr := s.resolver().Resolve(ctx, ref)
	if rerr != nil {
		h.Log("Warning: " + registryauth.Redact(rerr.Error()))
	} else if creds.Has() {
		h.Log(fmt.Sprintf("Using Docker credentials for %s (%s)", creds.Registry, creds.Source))
	}
	resp, err := cli.ImagePush(ctx, ref, client.ImagePushOptions{RegistryAuth: creds.HeaderValue()})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return registryAuthError(imageError(err), creds, ref, false)
	}
	defer resp.Close()
	return registryAuthError(consumePush(ctx, resp.JSONMessages(ctx), h), creds, ref, false)
}
