package appdb

import (
	"context"
	"encoding/json"
	"errors"

	"docklet/internal/model"
)

// MaxBuildsPerConnection is how many builds are kept per connection.
const MaxBuildsPerConnection = 50

// InsertBuild stores one build record and drops the oldest ones of that
// connection beyond MaxBuildsPerConnection. A record never contains argument
// values (the model has no field for them).
func (d *DB) InsertBuild(ctx context.Context, r model.BuildRecord) error {
	if r.ID == "" || r.ConnectionID == "" {
		return errors.New("build record needs an id and a connection")
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO build_history (id, connection_id, started_at, data) VALUES (?,?,?,?)`,
		r.ID, r.ConnectionID, r.StartedAt, string(data)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM build_history WHERE connection_id = ? AND id NOT IN
(SELECT id FROM build_history WHERE connection_id = ? ORDER BY started_at DESC, id DESC LIMIT ?)`,
		r.ConnectionID, r.ConnectionID, MaxBuildsPerConnection); err != nil {
		return err
	}
	return tx.Commit()
}

// ListBuilds returns the builds of one connection, newest first.
func (d *DB) ListBuilds(ctx context.Context, connectionID string) ([]model.BuildRecord, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT data FROM build_history WHERE connection_id = ? ORDER BY started_at DESC, id DESC`, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.BuildRecord{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		var r model.BuildRecord
		if json.Unmarshal([]byte(s), &r) == nil {
			out = append(out, r)
		}
	}
	return out, rows.Err()
}

// DeleteBuild removes one record.
func (d *DB) DeleteBuild(ctx context.Context, connectionID, id string) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM build_history WHERE connection_id = ? AND id = ?`, connectionID, id)
	return err
}

// ClearBuilds removes every record of one connection.
func (d *DB) ClearBuilds(ctx context.Context, connectionID string) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM build_history WHERE connection_id = ?`, connectionID)
	return err
}
