package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"docklet/internal/model"
)

type indexBuild struct {
	done    chan struct{}
	idx     *fileIndex
	err     error
	waiters int
	cancel  context.CancelFunc
}

type indexCache struct {
	mu       sync.Mutex
	items    map[string]*fileIndex
	order    []string // least recently used first
	gen      map[string]uint64
	inflight map[string]*indexBuild
}

func (c *indexCache) init() {
	if c.items == nil {
		c.items, c.gen, c.inflight = map[string]*fileIndex{}, map[string]uint64{}, map[string]*indexBuild{}
	}
}

func (c *indexCache) touch(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	c.order = append(c.order, key)
}

func (c *indexCache) dropLocked(key string) {
	delete(c.items, key)
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// cached returns a valid index for key/stamp or nil (dropping a stale one).
func (c *indexCache) cachedLocked(key, stamp string) *fileIndex {
	x := c.items[key]
	if x == nil {
		return nil
	}
	if x.stamp != stamp || time.Since(x.builtAt) > indexTTL {
		c.dropLocked(key)
		return nil
	}
	c.touch(key)
	return x
}

func (c *indexCache) peek(key, stamp string) *fileIndex {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.init()
	return c.cachedLocked(key, stamp)
}

func (c *indexCache) invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.init()
	c.dropLocked(key)
	c.gen[key]++
}

func indexKey(connectionID, containerID string) string { return connectionID + "/" + containerID }

// invalidateIndex drops the cached index of a container after this app changed its files.
func (s *FileService) invalidateIndex(connectionID, containerID string) {
	s.idx.invalidate(indexKey(connectionID, containerID))
}

// getIndex returns the container index, building it with one archive scan when missing,
// stale or force is set. Concurrent callers share one scan; the scan is cancelled when the
// last waiting caller goes away or the time budget is exceeded.
func (s *FileService) getIndex(ctx context.Context, api filesAPI, connectionID, containerID string, info containerInfo, force bool) (*fileIndex, error) {
	key, stamp := indexKey(connectionID, containerID), info.stamp()
	c := &s.idx
	c.mu.Lock()
	c.init()
	if force {
		c.dropLocked(key)
		c.gen[key]++
	} else if x := c.cachedLocked(key, stamp); x != nil {
		c.mu.Unlock()
		return x, nil
	}
	b := c.inflight[key]
	if b == nil {
		bctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), indexBuildBudget)
		b = &indexBuild{done: make(chan struct{}), cancel: cancel}
		c.inflight[key] = b
		gen := c.gen[key]
		go func() {
			started := time.Now()
			x, err := scanIndex(bctx, api, containerID)
			c.mu.Lock()
			defer c.mu.Unlock()
			if err == nil {
				x.stamp, x.builtAt, x.buildTook = stamp, time.Now(), time.Since(started)
				if c.gen[key] == gen {
					c.items[key] = x
					c.touch(key)
					for len(c.order) > indexCacheSize {
						c.dropLocked(c.order[0])
					}
				}
			} else if errors.Is(bctx.Err(), context.DeadlineExceeded) {
				err = errIndexUnavailable{fmt.Sprintf("scanning it took longer than %d seconds", int(indexBuildBudget.Seconds()))}
			}
			b.idx, b.err = x, err
			if c.inflight[key] == b {
				delete(c.inflight, key)
			}
			close(b.done)
			cancel()
		}()
	}
	b.waiters++
	c.mu.Unlock()

	select {
	case <-b.done:
		return b.idx, b.err
	case <-ctx.Done():
		c.mu.Lock()
		if b.waiters--; b.waiters == 0 {
			b.cancel()
		}
		c.mu.Unlock()
		return nil, friendlyError(ctx.Err())
	}
}

func scanIndex(ctx context.Context, api filesAPI, containerID string) (*fileIndex, error) {
	body, _, err := api.CopyFrom(ctx, containerID, "/")
	if err != nil {
		return nil, fileError(err)
	}
	defer body.Close()
	x, err := buildFileIndex(ctx, body)
	if err != nil {
		var unavailable errIndexUnavailable
		if errors.As(err, &unavailable) || ctx.Err() != nil {
			return nil, err
		}
		return nil, fileError(err)
	}
	return x, nil
}

// RefreshIndex drops the cached index of a container and scans it again. Only meaningful
// for containers that are listed through the index (stopped or without a usable shell).
func (s *FileService) RefreshIndex(ctx context.Context, connectionID, containerID string) (model.FileIndexInfo, error) {
	api, err := s.api(ctx, connectionID)
	if err != nil {
		return model.FileIndexInfo{}, err
	}
	info, err := api.Inspect(ctx, containerID)
	if err != nil {
		return model.FileIndexInfo{}, friendlyError(err)
	}
	x, err := s.getIndex(ctx, api, connectionID, containerID, info, true)
	if err != nil {
		return model.FileIndexInfo{}, err
	}
	return model.FileIndexInfo{Entries: x.entryCount, IndexedAt: x.builtAt.UnixMilli(), DurationMs: x.buildTook.Milliseconds()}, nil
}
