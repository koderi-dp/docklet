package services

import (
	"sort"
	"sync"
)

type statsWatches struct {
	mu sync.Mutex
	m  map[string]*statsWatch
}

func (r *statsWatches) add(w *statsWatch) {
	r.mu.Lock()
	if r.m == nil {
		r.m = map[string]*statsWatch{}
	}
	r.m[w.id] = w
	r.mu.Unlock()
}

func (r *statsWatches) remove(id string) {
	r.mu.Lock()
	delete(r.m, id)
	r.mu.Unlock()
}

func (r *statsWatches) get(id string) *statsWatch {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.m[id]
}

func (r *statsWatches) all() []*statsWatch {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*statsWatch, 0, len(r.m))
	for _, w := range r.m {
		out = append(out, w)
	}
	return out
}

func (r *statsWatches) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.m)
}

func normalizeIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
