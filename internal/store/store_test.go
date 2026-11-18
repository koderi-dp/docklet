package store

import (
	"os"
	"path/filepath"
	"testing"

	"docklet/internal/appdb"
)

func TestNewWithDB_ImportsLegacyJSONOnce(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "connections.json")
	os.WriteFile(legacy, []byte(`{"version":1,"connections":[{"id":"a","name":"A","type":"tcp","endpoint":"tcp://127.0.0.1:2375"}]}`), 0o600)
	db, err := appdb.Open(filepath.Join(dir, "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := NewWithDB(db, legacy)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.Load()
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("%+v", got)
	}
	if _, err := os.Stat(legacy); err == nil {
		t.Fatal("legacy file should be renamed")
	}
	if _, err := os.Stat(legacy + ".migrated"); err != nil {
		t.Fatal(err)
	}
	s2, _ := NewWithDB(db, legacy) // second run: nothing to import
	if got, _ := s2.Load(); len(got) != 1 {
		t.Fatal("re-import changed data")
	}
	if err := s.Save(nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Load(); len(got) != 0 {
		t.Fatal("save nil should clear")
	}
}
