CREATE TABLE IF NOT EXISTS admin_users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES admin_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS subscription_sources (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    fetch_fingerprint TEXT NOT NULL UNIQUE,
    url_ciphertext BLOB NOT NULL,
    url_nonce BLOB NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    last_refresh_at TEXT,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    source_node_key TEXT NOT NULL DEFAULT '',
    dedupe_fingerprint TEXT NOT NULL DEFAULT '',
    source_kind TEXT NOT NULL CHECK (source_kind IN ('manual', 'import', 'subscription')),
    subscription_source_id TEXT,
    protocol TEXT NOT NULL,
    server TEXT NOT NULL,
    server_port INTEGER NOT NULL,
    credential_ciphertext BLOB,
    credential_nonce BLOB,
    transport_json TEXT NOT NULL DEFAULT '{}',
    tls_json TEXT NOT NULL DEFAULT '{}',
    raw_payload_json TEXT NOT NULL DEFAULT '{}',
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    last_latency_ms INTEGER,
    last_status TEXT NOT NULL DEFAULT 'unknown' CHECK (last_status IN ('unknown', 'healthy', 'unreachable')),
    last_checked_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (subscription_source_id) REFERENCES subscription_sources(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_nodes_source ON nodes(subscription_source_id);
CREATE INDEX IF NOT EXISTS idx_nodes_dedupe_fingerprint ON nodes(dedupe_fingerprint);
CREATE UNIQUE INDEX IF NOT EXISTS idx_nodes_subscription_source_key
    ON nodes(subscription_source_id, source_node_key)
    WHERE subscription_source_id IS NOT NULL AND source_node_key <> '';

CREATE TABLE IF NOT EXISTS groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    filter_regex TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tunnels (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
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
    last_refresh_at TEXT,
    last_refresh_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE RESTRICT,
    FOREIGN KEY (current_node_id) REFERENCES nodes(id) ON DELETE SET NULL,
    UNIQUE (listen_host, listen_port)
);

CREATE INDEX IF NOT EXISTS idx_tunnels_group_id ON tunnels(group_id);
CREATE INDEX IF NOT EXISTS idx_tunnels_current_node_id ON tunnels(current_node_id);

CREATE TABLE IF NOT EXISTS tunnel_events (
    id TEXT PRIMARY KEY,
    tunnel_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    detail_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    FOREIGN KEY (tunnel_id) REFERENCES tunnels(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_tunnel_events_tunnel_id_created_at
    ON tunnel_events(tunnel_id, created_at DESC);

CREATE TABLE IF NOT EXISTS latency_samples (
    id TEXT PRIMARY KEY,
    node_id TEXT NOT NULL,
    tunnel_id TEXT,
    test_url TEXT NOT NULL,
    latency_ms INTEGER,
    success INTEGER NOT NULL CHECK (success IN (0, 1)),
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE,
    FOREIGN KEY (tunnel_id) REFERENCES tunnels(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_latency_samples_node_id_created_at
    ON latency_samples(node_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_latency_samples_tunnel_id_created_at
    ON latency_samples(tunnel_id, created_at DESC);
