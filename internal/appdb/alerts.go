package appdb

import (
	"context"

	"docklet/internal/model"
)

// alertsKeep bounds the persisted alert history.
const alertsKeep = 200

func init() {
	migrations = append(migrations, migration{10, `
CREATE TABLE alerts (
  id TEXT PRIMARY KEY,
  connection_id TEXT NOT NULL,
  connection_name TEXT NOT NULL DEFAULT '',
  container_id TEXT NOT NULL,
  container_name TEXT NOT NULL DEFAULT '',
  compose_project TEXT NOT NULL DEFAULT '',
  rule TEXT NOT NULL,
  severity TEXT NOT NULL,
  title TEXT NOT NULL,
  detail TEXT NOT NULL DEFAULT '',
  at INTEGER NOT NULL,
  read INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_alerts_at ON alerts (at DESC);
`})
}

// AddAlert stores an alert and trims the history to the newest alertsKeep rows.
func (d *DB) AddAlert(ctx context.Context, a model.Alert) error {
	_, err := d.db.ExecContext(ctx, `INSERT OR REPLACE INTO alerts
(id, connection_id, connection_name, container_id, container_name, compose_project, rule, severity, title, detail, at, read)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.ConnectionID, a.ConnectionName, a.ContainerID, a.ContainerName, a.ComposeProject,
		a.Rule, a.Severity, a.Title, a.Detail, a.At, a.Read)
	if err != nil {
		return err
	}
	_, err = d.db.ExecContext(ctx, `DELETE FROM alerts WHERE id NOT IN (SELECT id FROM alerts ORDER BY at DESC LIMIT ?)`, alertsKeep)
	return err
}

// ListAlerts returns the newest alerts first.
func (d *DB) ListAlerts(ctx context.Context, limit int) ([]model.Alert, error) {
	if limit <= 0 || limit > alertsKeep {
		limit = alertsKeep
	}
	rows, err := d.db.QueryContext(ctx, `SELECT id, connection_id, connection_name, container_id, container_name, compose_project,
rule, severity, title, detail, at, read FROM alerts ORDER BY at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Alert
	for rows.Next() {
		var a model.Alert
		if err := rows.Scan(&a.ID, &a.ConnectionID, &a.ConnectionName, &a.ContainerID, &a.ContainerName, &a.ComposeProject,
			&a.Rule, &a.Severity, &a.Title, &a.Detail, &a.At, &a.Read); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// MarkAlertsRead marks the given alerts read.
func (d *DB) MarkAlertsRead(ctx context.Context, ids []string) error {
	for _, id := range ids {
		if _, err := d.db.ExecContext(ctx, `UPDATE alerts SET read = 1 WHERE id = ?`, id); err != nil {
			return err
		}
	}
	return nil
}

// MarkAllAlertsRead marks every alert read.
func (d *DB) MarkAllAlertsRead(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, `UPDATE alerts SET read = 1`)
	return err
}

// ClearAlerts deletes all alerts.
func (d *DB) ClearAlerts(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM alerts`)
	return err
}
