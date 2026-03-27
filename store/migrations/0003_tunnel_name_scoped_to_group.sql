PRAGMA foreign_keys = OFF;

CREATE TABLE tunnels_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    group_id TEXT NOT NULL,
    listen_host TEXT NOT NULL DEFAULT '127.0.0.1',
    listen_port INTEGER NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('stopped', 'starting', 'running', 'degraded', 'error')),
    current_node_id TEXT,
    auth_username_ciphertext BLOB,
    auth_password_ciphertext BLOB,
    auth_nonce BLOB,
    controller_port INTEGER NOT NULL UNIQUE,
    controller_secret_ciphertext BLOB NOT NULL,
    controller_secret_nonce BLOB NOT NULL,
    runtime_dir TEXT NOT NULL,
    runtime_config_json TEXT NOT NULL DEFAULT '',
    last_refresh_at TEXT,
    last_refresh_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE RESTRICT,
    FOREIGN KEY (current_node_id) REFERENCES nodes(id) ON DELETE SET NULL,
    UNIQUE (group_id, name),
    UNIQUE (listen_host, listen_port)
);

INSERT INTO tunnels_new (
    id, name, group_id, listen_host, listen_port, status, current_node_id,
    auth_username_ciphertext, auth_password_ciphertext, auth_nonce,
    controller_port, controller_secret_ciphertext, controller_secret_nonce,
    runtime_dir, runtime_config_json, last_refresh_at, last_refresh_error,
    created_at, updated_at
)
SELECT
    id, name, group_id, listen_host, listen_port, status, current_node_id,
    auth_username_ciphertext, auth_password_ciphertext, auth_nonce,
    controller_port, controller_secret_ciphertext, controller_secret_nonce,
    runtime_dir, runtime_config_json, last_refresh_at, last_refresh_error,
    created_at, updated_at
FROM tunnels;

DROP TABLE tunnels;
ALTER TABLE tunnels_new RENAME TO tunnels;

CREATE INDEX idx_tunnels_group_id ON tunnels(group_id);
CREATE INDEX idx_tunnels_current_node_id ON tunnels(current_node_id);

PRAGMA foreign_keys = ON;
