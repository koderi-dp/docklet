package appdb

import (
	"context"
	"database/sql"
	"errors"

	"docklet/internal/model"
)

// GetSetting returns a setting value; ok=false when unset.
func (d *DB) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := d.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

// SetSetting upserts a setting.
func (d *DB) SetSetting(ctx context.Context, key, value string) error {
	_, err := d.db.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// RecordOperation stores operation metadata (no logs). Upserts by ID.
func (d *DB) RecordOperation(ctx context.Context, op model.Operation) error {
	_, err := d.db.ExecContext(ctx, `INSERT INTO operation_history (id, kind, title, connection_id, resource, status, error, started_at, finished_at)
VALUES (?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET status=excluded.status, error=excluded.error, finished_at=excluded.finished_at, title=excluded.title`,
		op.ID, op.Kind, op.Title, op.ConnectionID, op.Resource, string(op.Status), op.Error, op.StartedAt, op.FinishedAt)
	return err
}

// ListOperationHistory returns the most recent operations first.
func (d *DB) ListOperationHistory(ctx context.Context, limit int) ([]model.Operation, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := d.db.QueryContext(ctx, `SELECT id, kind, title, connection_id, resource, status, error, started_at, finished_at
FROM operation_history ORDER BY CASE WHEN finished_at=0 THEN started_at ELSE finished_at END DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Operation
	for rows.Next() {
		var o model.Operation
		var st string
		if err := rows.Scan(&o.ID, &o.Kind, &o.Title, &o.ConnectionID, &o.Resource, &st, &o.Error, &o.StartedAt, &o.FinishedAt); err != nil {
			return nil, err
		}
		o.Status = model.OperationStatus(st)
		out = append(out, o)
	}
	return out, rows.Err()
}
