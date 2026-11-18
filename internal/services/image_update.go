package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/distribution/reference"
	"github.com/moby/moby/client"

	"docklet/internal/model"
	"docklet/internal/registryauth"
)

// updateCacheTTL bounds how often the registry is asked about the same tag.
const updateCacheTTL = 15 * time.Minute

// updateCheckTimeout bounds one registry round trip.
const updateCheckTimeout = 20 * time.Second

// updateCache keeps verdicts per connection+ref+local digests (so pulling a new
// image changes the key and never shows a stale "update available").
type updateCache struct {
	mu  sync.Mutex
	now func() time.Time // nil = time.Now
	m   map[string]cachedUpdate
}

type cachedUpdate struct {
	st model.ImageUpdateStatus
	at time.Time
}

func (c *updateCache) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (c *updateCache) get(key string) (model.ImageUpdateStatus, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok || c.clock().Sub(e.at) >= updateCacheTTL {
		delete(c.m, key)
		return model.ImageUpdateStatus{}, false
	}
	return e.st, true
}

func (c *updateCache) put(key string, st model.ImageUpdateStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]cachedUpdate{}
	}
	c.m[key] = cachedUpdate{st: st, at: c.clock()}
}

// CheckUpdate asks the registry (through the Engine, which honours its registry
// configuration) whether the tag of ref now points to a different manifest than
// the local copy. It always contacts the registry; per-image problems (no
// credentials, not from a registry, offline) are reported in Status.Error.
func (s *ImageService) CheckUpdate(ctx context.Context, connectionID, ref string) (model.ImageUpdateStatus, error) {
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return model.ImageUpdateStatus{}, err
	}
	return s.checkOne(ctx, cli, connectionID, ref, false), nil
}

// CheckUpdates checks many refs with bounded concurrency; results are served from
// a 15-minute cache. The result has one entry per ref, in order.
func (s *ImageService) CheckUpdates(ctx context.Context, connectionID string, refs []string) ([]model.ImageUpdateStatus, error) {
	cli, err := s.client(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	out := make([]model.ImageUpdateStatus, len(refs))
	forEachBounded(ctx, len(refs), func(i int) {
		out[i] = s.checkOne(ctx, cli, connectionID, refs[i], true)
	}, func(i int) {
		out[i] = model.ImageUpdateStatus{Ref: refs[i], CheckedAt: time.Now().UnixMilli(), Error: "Check cancelled"}
	})
	return out, nil
}

func (s *ImageService) checkOne(ctx context.Context, cli *client.Client, connectionID, ref string, useCache bool) model.ImageUpdateStatus {
	st := model.ImageUpdateStatus{Ref: ref, CheckedAt: s.updates.clock().UnixMilli()}
	fail := func(msg string) model.ImageUpdateStatus { st.Error = msg; return st }

	named, err := updateRefName(ref)
	if err != nil {
		return fail(err.Error())
	}
	ctx, cancel := context.WithTimeout(ctx, updateCheckTimeout)
	defer cancel()
	ins, err := cli.ImageInspect(ctx, named.String())
	if err != nil {
		return fail(imageError(err).Error())
	}
	local := localDigestsFor(ins.RepoDigests, named)
	if len(local) == 0 {
		return fail("Not from a registry: this image has no registry digest (built or loaded locally), so there is nothing to compare.")
	}
	st.LocalDigest = strings.Join(local, ", ")

	key := connectionID + "|" + named.String() + "|" + st.LocalDigest
	if useCache {
		if c, ok := s.updates.get(key); ok {
			return c
		}
	}
	creds, _ := s.resolver().Resolve(ctx, named.String()) // unresolved credentials: try anonymously
	res, err := cli.DistributionInspect(ctx, named.String(), client.DistributionInspectOptions{EncodedRegistryAuth: creds.HeaderValue()})
	if err != nil {
		return fail(updateCheckError(err, creds, named.String()))
	}
	st.RemoteDigest = res.Descriptor.Digest.String()
	st.UpdateAvailable = !containsString(local, st.RemoteDigest)
	s.updates.put(key, st)
	return st
}

// updateRefName validates ref and returns it as a tagged name (no tag = latest).
// References pinned by digest cannot have updates.
func updateRefName(ref string) (reference.Named, error) {
	ref, err := validateRepoTag(ref)
	if err != nil {
		return nil, err
	}
	n, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return nil, err
	}
	if _, ok := n.(reference.Digested); ok {
		return nil, errors.New("Pinned by digest: this reference always points to the same image, so it has no updates.")
	}
	return reference.TagNameOnly(n), nil
}

// localDigestsFor returns the digests ("sha256:...") of RepoDigests entries that
// belong to the same repository as name.
func localDigestsFor(repoDigests []string, name reference.Named) []string {
	var out []string
	for _, rd := range repoDigests {
		d, err := reference.ParseNormalizedNamed(rd)
		if err != nil {
			continue
		}
		if dig, ok := d.(reference.Digested); ok && d.Name() == name.Name() {
			out = append(out, dig.Digest().String())
		}
	}
	return out
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func updateCheckError(err error, creds registryauth.Credentials, ref string) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "The registry did not answer in time."
	}
	low := strings.ToLower(err.Error())
	switch {
	case strings.Contains(low, "toomanyrequests"), strings.Contains(low, "rate limit"), strings.Contains(low, "429"):
		return "The registry is rate limiting requests. Try again later."
	case isAuthFailure(err):
		return registryAuthError(err, creds, ref, false).Error()
	}
	return fmt.Sprint(imageError(err))
}
