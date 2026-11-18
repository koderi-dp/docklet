package appdb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// ComposeProjectRecord is a managed Compose project saved per connection.
type ComposeProjectRecord struct {
	ID           string   `json:"id"`
	ConnectionID string   `json:"connectionId"`
	Name         string   `json:"name"`
	WorkingDir   string   `json:"workingDir"`
	ConfigFiles  []string `json:"configFiles"`
	EnvFiles     []string `json:"envFiles"`
	CreatedAt    int64    `json:"createdAt"`
	UpdatedAt    int64    `json:"updatedAt"`
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func encList(l []string) string {
	if l == nil {
		l = []string{}
	}
	b, _ := json.Marshal(l)
	return string(b)
}

func decList(s string) []string {
	var l []string
	_ = json.Unmarshal([]byte(s), &l)
	return l
}

// ListComposeProjects returns the projects of one connection ordered by name.
func (d *DB) ListComposeProjects(ctx context.Context, connectionID string) ([]ComposeProjectRecord, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT id, connection_id, name, working_dir, config_files, env_files, created_at, updated_at
FROM compose_projects WHERE connection_id = ? ORDER BY name`, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ComposeProjectRecord
	for rows.Next() {
		var r ComposeProjectRecord
		var cf, ef string
		if err := rows.Scan(&r.ID, &r.ConnectionID, &r.Name, &r.WorkingDir, &cf, &ef, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.ConfigFiles, r.EnvFiles = decList(cf), decList(ef)
		out = append(out, r)
	}
	return out, rows.Err()
}

// SaveComposeProject upserts by (connectionID, name) and assigns an ID.
func (d *DB) SaveComposeProject(ctx context.Context, r ComposeProjectRecord) (ComposeProjectRecord, error) {
	if r.ConnectionID == "" || r.Name == "" {
		return r, errors.New("connection and project name are required")
	}
	now := time.Now().UnixMilli()
	var id string
	var created int64
	err := d.db.QueryRowContext(ctx, `SELECT id, created_at FROM compose_projects WHERE connection_id = ? AND name = ?`, r.ConnectionID, r.Name).Scan(&id, &created)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		r.ID, r.CreatedAt = newID(), now
	case err != nil:
		return r, err
	default:
		r.ID, r.CreatedAt = id, created
	}
	r.UpdatedAt = now
	_, err = d.db.ExecContext(ctx, `INSERT INTO compose_projects (id, connection_id, name, working_dir, config_files, env_files, created_at, updated_at)
VALUES (?,?,?,?,?,?,?,?)
ON CONFLICT(connection_id, name) DO UPDATE SET working_dir=excluded.working_dir, config_files=excluded.config_files, env_files=excluded.env_files, updated_at=excluded.updated_at`,
		r.ID, r.ConnectionID, r.Name, r.WorkingDir, encList(r.ConfigFiles), encList(r.EnvFiles), r.CreatedAt, r.UpdatedAt)
	return r, err
}

// DeleteComposeProject removes a saved project (no error when absent).
func (d *DB) DeleteComposeProject(ctx context.Context, connectionID, name string) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM compose_projects WHERE connection_id = ? AND name = ?`, connectionID, name)
	return err
}
