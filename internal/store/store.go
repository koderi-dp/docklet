// Package store persists user-saved connection profiles in the application
// SQLite database (internal/appdb). Profiles never contain secrets (see
// model.ConnectionProfile); secrets live in the OS credential store.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"docklet/internal/appdb"
	"docklet/internal/model"
)

// Store is the SQLite-backed profile store (implements engine.ProfileStore).
type Store struct {
	mu sync.Mutex
	db *appdb.DB

	// lazy mode (New): database opened on first use next to the legacy file.
	lazyDir, lazyLegacy string
	lazyErr             error
}

// New keeps the original constructor: path is the legacy connections.json
// location (as returned by DefaultPath). The SQLite database is opened lazily as
// docklet.db next to it and the JSON file is imported on first use. Prefer
// NewWithDB when the application already opened the database.
func New(legacyJSON string) *Store {
	return &Store{lazyDir: filepath.Dir(legacyJSON), lazyLegacy: legacyJSON}
}

// NewWithDB returns a store backed by db. When legacyJSON names an existing
// connections.json it is imported once (only into an empty table) and renamed
// to connections.json.migrated. Import problems are returned but the store is
// still usable.
func NewWithDB(db *appdb.DB, legacyJSON string) (*Store, error) {
	s := &Store{db: db}
	return s, s.importLegacy(legacyJSON)
}

// DefaultPath is the legacy os.UserConfigDir()/docklet/connections.json path.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "docklet", "connections.json"), nil
}

func (s *Store) open() (*appdb.DB, error) {
	if s.db != nil {
		return s.db, nil
	}
	if s.lazyErr != nil {
		return nil, s.lazyErr
	}
	db, err := appdb.Open(filepath.Join(s.lazyDir, "docklet.db"))
	if err != nil {
		s.lazyErr = err
		return nil, err
	}
	s.db = db
	_ = s.importLegacy(s.lazyLegacy) // import problems must not block the store
	return db, nil
}

type legacyFormat struct {
	Version     int                       `json:"version"`
	Connections []model.ConnectionProfile `json:"connections"`
}

func (s *Store) importLegacy(path string) error {
	if path == "" {
		return nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var f legacyFormat
	if err := json.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("connections file %s is corrupt and was not imported: %w", path, err)
	}
	ctx := context.Background()
	existing, err := s.db.LoadConnections(ctx)
	if err != nil {
		return err
	}
	if len(existing) == 0 && len(f.Connections) > 0 {
		for i := range f.Connections {
			f.Connections[i].Discovered = false
		}
		if err := s.db.ReplaceConnections(ctx, f.Connections); err != nil {
			return err
		}
	}
	return os.Rename(path, path+".migrated")
}

// Load returns the saved profiles (empty when none).
func (s *Store) Load() ([]model.ConnectionProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	return db.LoadConnections(context.Background())
}

// Save atomically replaces the saved profiles.
func (s *Store) Save(profiles []model.ConnectionProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	db, err := s.open()
	if err != nil {
		return err
	}
	return db.ReplaceConnections(context.Background(), profiles)
}
