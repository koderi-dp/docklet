package appdb

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type migration struct {
	ID  int
	SQL string
}

// migrations is the ordered, append-only schema history.
var migrations = []migration{
	{1, `
CREATE TABLE connections (
  id TEXT PRIMARY KEY,
  position INTEGER NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  endpoint TEXT NOT NULL,
  wsl_distro TEXT NOT NULL DEFAULT '',
  tls_verify INTEGER NOT NULL DEFAULT 0,
  tls_cert_dir TEXT NOT NULL DEFAULT '',
  allow_insecure INTEGER NOT NULL DEFAULT 0,
  ssh_user TEXT NOT NULL DEFAULT '',
  ssh_port INTEGER NOT NULL DEFAULT 0,
  ssh_identity_file TEXT NOT NULL DEFAULT '',
  ssh_use_agent INTEGER NOT NULL DEFAULT 0,
  ssh_proxy_jump TEXT NOT NULL DEFAULT ''
);
CREATE TABLE compose_projects (
  id TEXT PRIMARY KEY,
  connection_id TEXT NOT NULL,
  name TEXT NOT NULL,
  working_dir TEXT NOT NULL DEFAULT '',
  config_files TEXT NOT NULL DEFAULT '[]',
  env_files TEXT NOT NULL DEFAULT '[]',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  UNIQUE (connection_id, name)
);
CREATE TABLE settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE operation_history (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  title TEXT NOT NULL,
  connection_id TEXT NOT NULL DEFAULT '',
  resource TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  started_at INTEGER NOT NULL,
  finished_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_operation_history_finished ON operation_history (finished_at DESC);
`},
}

func migrate(ctx context.Context, db *sql.DB, list []migration) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (id INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return err
	}
	applied := map[int]bool{}
	maxApplied := 0
	rows, err := db.QueryContext(ctx, `SELECT id FROM schema_migrations`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		applied[id] = true
		if id > maxApplied {
			maxApplied = id
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	known := map[int]bool{}
	for _, m := range list {
		known[m.ID] = true
	}
	for id := range applied {
		if !known[id] {
			return fmt.Errorf("the database schema (version %d) is newer than this version of Docklet; update the app", maxApplied)
		}
	}
	for _, m := range list {
		if applied[m.ID] {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d failed: %w", m.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (id, applied_at) VALUES (?, ?)`, m.ID, time.Now().UnixMilli()); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
