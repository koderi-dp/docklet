package registryauth

import (
	"encoding/json"
	"errors"
	"sort"
	"sync"

	"docklet/internal/secrets"
)

// Docklet keeps credentials it saves itself in the OS credential store: one
// entry per registry plus one index entry (host and user name, no secret) so
// the registries can be listed without reading any secret.
const (
	storeIndexKey  = "registry/index"
	storeEntryPref = "registry/"
)

var storeMu sync.Mutex

type storedSecret struct {
	Username string `json:"u"`
	Secret   string `json:"s"`
}

// DockletEntry is a registry sign-in kept by Docklet (no secret).
type DockletEntry struct {
	Host     string `json:"host"`
	Username string `json:"username"`
}

func entryKey(host string) string { return storeEntryPref + host }

func readIndex(st secrets.Store) (map[string]string, error) {
	idx := map[string]string{}
	v, err := st.Get(storeIndexKey)
	if errors.Is(err, secrets.ErrNotFound) || (err == nil && v == "") {
		return idx, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(v), &idx); err != nil {
		return map[string]string{}, nil // damaged index: rebuilt as entries are saved
	}
	return idx, nil
}

func writeIndex(st secrets.Store, idx map[string]string) error {
	if len(idx) == 0 {
		return st.Delete(storeIndexKey)
	}
	b, _ := json.Marshal(idx)
	return st.Set(storeIndexKey, string(b))
}

// SaveDocklet stores credentials for a normalized host in the OS credential store.
func SaveDocklet(st secrets.Store, host, username, secret string) error {
	if st == nil {
		return errors.New("the system credential store is not available")
	}
	host = NormalizeHost(host)
	b, _ := json.Marshal(storedSecret{Username: username, Secret: secret})
	storeMu.Lock()
	defer storeMu.Unlock()
	if err := st.Set(entryKey(host), string(b)); err != nil {
		return errors.New("could not save to the system credential store: " + Redact(err.Error()))
	}
	idx, err := readIndex(st)
	if err != nil {
		idx = map[string]string{}
	}
	idx[host] = username
	if err := writeIndex(st, idx); err != nil {
		return errors.New("could not update the credential index: " + Redact(err.Error()))
	}
	return nil
}

// DeleteDocklet removes Docklet's entry for host (missing is fine).
func DeleteDocklet(st secrets.Store, host string) error {
	if st == nil {
		return nil
	}
	host = NormalizeHost(host)
	storeMu.Lock()
	defer storeMu.Unlock()
	if err := st.Delete(entryKey(host)); err != nil {
		return errors.New("could not remove the credential: " + Redact(err.Error()))
	}
	idx, err := readIndex(st)
	if err != nil {
		return nil
	}
	if _, ok := idx[host]; ok {
		delete(idx, host)
		return writeIndex(st, idx)
	}
	return nil
}

// ListDocklet returns the registries Docklet holds credentials for, sorted by host.
func ListDocklet(st secrets.Store) ([]DockletEntry, error) {
	if st == nil {
		return nil, nil
	}
	storeMu.Lock()
	defer storeMu.Unlock()
	idx, err := readIndex(st)
	if err != nil {
		return nil, errors.New("could not read the system credential store: " + Redact(err.Error()))
	}
	out := make([]DockletEntry, 0, len(idx))
	for h, u := range idx {
		out = append(out, DockletEntry{Host: h, Username: u})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out, nil
}

func (r *Resolver) fromDockletStore(host string) (Credentials, bool) {
	if r.Store == nil {
		return Credentials{}, false
	}
	v, err := r.Store.Get(entryKey(host))
	if err != nil || v == "" {
		return Credentials{}, false
	}
	var e storedSecret
	if json.Unmarshal([]byte(v), &e) != nil || e.Secret == "" {
		return Credentials{}, false
	}
	c := Credentials{Registry: host, Source: SourceDocklet, Username: e.Username, Password: e.Secret}
	if e.Username == "<token>" {
		c.Username, c.Password, c.IdentityToken = "", "", e.Secret
	}
	return c, true
}
