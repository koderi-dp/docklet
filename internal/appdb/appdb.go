// Package appdb is the application's SQLite database (pure-Go driver). It holds
// connection profiles, saved Compose projects, settings and operation history
// metadata. It never stores secrets or logs.
package appdb

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite handle.
type DB struct {
	db *sql.DB
}

// DefaultPath is os.UserConfigDir()/docklet/docklet.db.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "docklet", "docklet.db"), nil
}

// Open creates parent directories, opens the database in WAL mode with a busy
// timeout and a single connection (single writer), and applies migrations.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // Windows drive paths: file:/C:/...
	}
	u := url.URL{Scheme: "file", Path: p}
	dsn := u.String() + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	sdb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sdb.SetMaxOpenConns(1)
	if err := sdb.Ping(); err != nil {
		sdb.Close()
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	if err := migrate(context.Background(), sdb, migrations); err != nil {
		sdb.Close()
		return nil, err
	}
	return &DB{db: sdb}, nil
}

// Close closes the database.
func (d *DB) Close() error { return d.db.Close() }

// SQL exposes the handle for area-specific stores.
func (d *DB) SQL() *sql.DB { return d.db }
