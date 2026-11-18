package services

import (
	"bufio"
	"context"
	"docklet/internal/model"
	"docklet/internal/ops"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Build builds an image from a local context directory as a background
// operation. The context is read on this computer and uploaded to the Engine
// (the classic builder of the Engine API; BuildKit-only features are not used).
func (s *ImageService) Build(ctx context.Context, connectionID string, spec model.BuildSpec) (model.Operation, error) {
	v, err := validateBuildSpec(spec)
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
	title := "Build " + filepath.Base(v.ContextDir)
	resource := filepath.Base(v.ContextDir)
	if len(v.Tags) > 0 {
		title, resource = "Build "+v.Tags[0], v.Tags[0]
	}
	op := s.Ops.Start(ops.Spec{
		Kind: "image.build", Title: title, ConnectionID: connectionID, Resource: resource, Cancellable: true,
	}, func(octx context.Context, h *ops.Handle) error {
		err := s.runBuild(octx, cli, v, h)
		s.emitChanged(connectionID)
		return err
	})
	return op, nil
}

func (s *ImageService) runBuild(ctx context.Context, cli *client.Client, v validatedBuild, h *ops.Handle) error {
	h.Log("Preparing build context from " + v.ContextDir)
	entries, err := buildContextEntries(v.ContextDir, v.Dockerfile)
	if err != nil {
		return err
	}
	h.Log(fmt.Sprintf("Uploading %d files to the Engine", len(entries)))

	opts := client.ImageBuildOptions{
		Tags: v.Tags, Dockerfile: v.Dockerfile, BuildArgs: v.BuildArgs, Target: v.Target,
		NoCache: v.NoCache, PullParent: v.Pull, Remove: true, ForceRemove: true,
	}
	if v.Platform != nil {
		opts.Platforms = []ocispec.Platform{*v.Platform}
	}
	// Credentials for private base images (FROM), from the Docker CLI config.
	if df, err := os.ReadFile(filepath.Join(v.ContextDir, filepath.FromSlash(v.Dockerfile))); err == nil {
		for _, host := range baseImageRegistries(df) {
			c, rerr := s.resolver().ResolveHost(ctx, host)
			if rerr != nil || !c.Has() {
				continue
			}
			if opts.AuthConfigs == nil {
				opts.AuthConfigs = map[string]registry.AuthConfig{}
			}
			opts.AuthConfigs[c.ServerAddress()] = registry.AuthConfig{
				Username: c.Username, Password: c.Password, IdentityToken: c.IdentityToken, ServerAddress: c.ServerAddress(),
			}
			h.Log(fmt.Sprintf("Using Docker credentials for base image registry %s (%s)", host, c.Source))
		}
	}

	body := openBuildContext(ctx, v.ContextDir, entries)
	defer body.Close()
	res, err := cli.ImageBuild(ctx, body, opts)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return imageError(err)
	}
	defer res.Body.Close()
	stop := context.AfterFunc(ctx, func() { res.Body.Close() })
	defer stop()
	return consumeBuild(ctx, res.Body, h)
}

var stepRe = regexp.MustCompile(`^Step (\d+)/(\d+)\b`)

// consumeBuild parses the Engine's build JSON stream: "stream" lines become log
// lines, "errorDetail" fails the build, and "Step n/m" lines give real progress.
func consumeBuild(ctx context.Context, r io.Reader, h pullSink) error {
	dec := json.NewDecoder(bufio.NewReader(r))
	var imageID string
	for {
		var m jsonstream.Message
		err := dec.Decode(&m)
		if err == io.EOF {
			break
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return imageError(fmt.Errorf("build output interrupted: %w", err))
		}
		if m.Error != nil {
			msg := strings.TrimSpace(m.Error.Message)
			if msg == "" {
				msg = "build failed"
			}
			if m.Error.Code != 0 && !strings.Contains(msg, strconv.Itoa(m.Error.Code)) {
				msg += " (exit code " + strconv.Itoa(m.Error.Code) + ")"
			}
			return errors.New("Build failed: " + msg)
		}
		if m.Stream != "" {
			for _, line := range strings.Split(strings.ReplaceAll(m.Stream, "\r\n", "\n"), "\n") {
				line = strings.TrimRight(line, " \r")
				if line == "" {
					continue
				}
				h.Log(line)
				if sm := stepRe.FindStringSubmatch(line); sm != nil {
					n, _ := strconv.Atoi(sm[1])
					total, _ := strconv.Atoi(sm[2])
					if total > 0 {
						h.Progress(float64(n-1) / float64(total))
						h.Detail(line)
					}
				}
			}
		} else if m.Status != "" {
			h.Log(strings.TrimSpace(m.Status + " " + m.ID))
		}
		if m.Aux != nil {
			var aux struct{ ID string }
			if json.Unmarshal(*m.Aux, &aux) == nil && aux.ID != "" {
				imageID = aux.ID
			}
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if imageID != "" {
		h.Log("Built image " + imageID)
	}
	h.Progress(1)
	return nil
}
