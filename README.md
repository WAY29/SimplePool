# SimplePool

[简体中文](./README.zh-CN.md)

SimplePool is built on top of `sing-box` and follows a simple idea: filter nodes into groups first, then create HTTP tunnels from those groups. Once a tunnel starts, it stays on one selected node until you explicitly refresh it to another available node.

That design comes from a practical gap in Clash-style load balancing. Some workflows do not want a different node per request or per destination. They need the same node for the whole run. SimplePool lets you enable or disable nodes inside a group, create tunnels dynamically from that group, and refresh the current tunnel node only when you decide to switch.

## Screenshots

| Login | Node Pool |
| --- | --- |
| ![SimplePool login screen](./reference/login.png) | ![SimplePool node pool screen](./reference/nodes.png) |

## Highlights

- Single binary backend with an embedded React web UI.
- Manual nodes, payload import, and subscription sources in one place.
- Filter nodes by group first, then create HTTP tunnels from that group.
- One isolated embedded `sing-box` runtime per HTTP tunnel, with a stable selected node.
- Enable or disable nodes at any time, then manually refresh a tunnel to move to another available node.
- More flexible than per-request balancing when an entire workflow must stay on one exit node.
- SQLite-backed state, encrypted secrets, runtime event history, and tunnel lifecycle management.
- Debug-mode OpenAPI output for API inspection and integration.

## Scope

- HTTP tunnels only.
- Single admin, single machine, self-hosted deployment.
- No multi-tenant scheduling.
- No automatic failover.
- No periodic auto-optimization.

## Quick Start

### Prerequisites

- `Go 1.25.6`
- `Node.js 22.12.0`
- `npm`
- `mise` is recommended for toolchain management

### Install dependencies

```bash
git clone https://github.com/WAY29/SimplePool
cd SimplePool
mise install
npm --prefix web install
cp .env.example .env
```

### Configure the environment

Edit `.env` before starting the app.

- `SIMPLEPOOL_ADMIN_PASSWORD` is required.
- `SIMPLEPOOL_MASTER_KEY` or `SIMPLEPOOL_MASTER_KEY_FILE` is required, and they cannot both be set.
- The example key in `.env.example` is for local development only. Replace it before real use.
- `SIMPLEPOOL_LOG_LEVEL` controls both SimplePool logs and the embedded `sing-box` runtime logs, including tunnel `stdout.log` and `stderr.log`.
- The default listen address is `127.0.0.1:7891`.

### Build the embedded web UI and run the server

```bash
mise run web:build
go run -tags 'with_quic with_dhcp with_wireguard with_clash_api' ./cmd/simplepool-api --config .env
```

Open `http://127.0.0.1:7891` in your browser.

To expose `openapi.json`, start the server in debug mode:

```bash
go run -tags 'with_quic with_dhcp with_wireguard with_clash_api' ./cmd/simplepool-api --config .env -debug
```

### Build a release binary

```bash
mise run build
./simplepool --config .env
```

## Usage

1. Sign in with the admin account from your config file.
2. Add manual nodes, import a payload, or create subscription sources.
3. Create regex-based groups to shape tunnel candidate pools.
4. Create HTTP tunnels from a group and let SimplePool pick a usable node.
5. Use `Refresh` to lock a new usable node, or `Start` / `Stop` to manage the tunnel runtime.

Basic health check:

```bash
curl http://127.0.0.1:7891/healthz
```

Minimal login example:

```bash
curl \
  -X POST http://127.0.0.1:7891/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"change-me"}'
```

## Documentation

- [Architecture](./docs/ARCHITECTURE.md)
- [Development](./docs/DEVELOPMENT.md)
- [Bundled OpenAPI JSON](./internal/httpapi/openapi/openapi.json)
- Runtime endpoint: `GET /openapi.json` when started with `-debug`

## Contributing

Issues and pull requests are welcome.

Recommended local checks:

```bash
mise run check
npm --prefix web run test
```

For frontend-only iteration, keep the backend running on `127.0.0.1:7891`, then start Vite in another terminal:

```bash
npm --prefix web run dev
```

## License

[GNU General Public License v3.0 or later](./LICENSE)
