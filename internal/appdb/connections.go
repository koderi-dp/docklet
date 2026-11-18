package appdb

import (
	"context"

	"docklet/internal/model"
)

// LoadConnections returns saved profiles in their stored order.
func (d *DB) LoadConnections(ctx context.Context) ([]model.ConnectionProfile, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT id, name, type, endpoint, wsl_distro, tls_verify, tls_cert_dir, allow_insecure,
ssh_user, ssh_port, ssh_identity_file, ssh_use_agent, ssh_proxy_jump FROM connections ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ConnectionProfile
	for rows.Next() {
		var p model.ConnectionProfile
		var typ string
		if err := rows.Scan(&p.ID, &p.Name, &typ, &p.Endpoint, &p.WSLDistro, &p.TLSVerify, &p.TLSCertDir, &p.AllowInsecure,
			&p.SSHUser, &p.SSHPort, &p.SSHIdentityFile, &p.SSHUseAgent, &p.SSHProxyJump); err != nil {
			return nil, err
		}
		p.Type = model.ConnectionType(typ)
		out = append(out, p)
	}
	return out, rows.Err()
}

// ReplaceConnections atomically replaces all saved profiles (order preserved).
func (d *DB) ReplaceConnections(ctx context.Context, profiles []model.ConnectionProfile) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM connections`); err != nil {
		return err
	}
	for i, p := range profiles {
		if _, err := tx.ExecContext(ctx, `INSERT INTO connections (id, position, name, type, endpoint, wsl_distro, tls_verify, tls_cert_dir,
allow_insecure, ssh_user, ssh_port, ssh_identity_file, ssh_use_agent, ssh_proxy_jump) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			p.ID, i, p.Name, string(p.Type), p.Endpoint, p.WSLDistro, p.TLSVerify, p.TLSCertDir, p.AllowInsecure,
			p.SSHUser, p.SSHPort, p.SSHIdentityFile, p.SSHUseAgent, p.SSHProxyJump); err != nil {
			return err
		}
	}
	return tx.Commit()
}
