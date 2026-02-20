# Multi-App Mode Design

**Goal:** Matcha manages multiple Docker apps on a single server with shared kamal-proxy, zero-downtime deploys, and per-app backups.

**Architecture:** One shared kamal-proxy routes by hostname to N app containers on a shared Docker network. Two entry points: embedded Go library (your own apps) and standalone CLI (third-party images).

## Architecture

```
Internet → matcha-proxy (ports 80/443, TLS) → app containers (internal ports)
                 ↓
         routes by hostname
         ↓              ↓              ↓
    fusionaly:8080  plausible:8000  gitea:3000
```

One server runs:
- **One `matcha-proxy` container** — kamal-proxy on ports 80/443, shared Docker network
- **N app containers** — each on its own internal port, all on `matcha-network`
- **One app registry** at `/etc/matcha/` tracking what's deployed

kamal-proxy routes by `Host` header. Each app gets its own domain. TLS is automatic per-domain via Let's Encrypt.

Shared infra (Docker, proxy, network) is set up lazily — the first `install` or `matcha setup` creates it. Subsequent apps just register themselves.

## Two Entry Points, One System

### Entry Point 1: Embedded binary (your own apps)

Each app embeds Matcha as a Go library with baked-in config. The public API stays the same:

```go
m := matcha.New(matcha.Config{
    Name:     "fusionaly",
    AppImage: "ghcr.io/karloscodes/fusionaly:latest",
    AppPort:  8080,
    HealthPath: "/up",
    Backups: true,
})

switch os.Args[1] {
case "install":
    m.Install()
case "update":
    m.Update()
}
```

Running on a fresh server:

```
$ fusionaly install
Domain: app.fusionaly.com
Checking DNS... ✓

Installing Docker...        ✓
Starting matcha-proxy...     ✓
Pulling fusionaly image...   ✓
Starting fusionaly...        ✓
Registering with proxy...    ✓

Done! https://app.fusionaly.com
```

Subsequent updates:

```
$ fusionaly update
Pulling latest image...     ✓
Redeploying...              ✓    ← zero-downtime via kamal-proxy

Done!
```

### Entry Point 2: Standalone CLI (third-party images)

For deploying other people's Docker images without writing Go code.

**Setup (once per server):**

```
$ matcha setup
Installing Docker...        ✓
Creating matcha-network...   ✓
Starting matcha-proxy...     ✓

Matcha is ready. Add your first app with 'matcha add'.
```

**Add and deploy apps:**

```
$ matcha add plausible \
    --image plausible/analytics:latest \
    --domain analytics.example.com \
    --port 8000 \
    --volume /data:/var/lib/plausible/db

App 'plausible' added. Edit env vars: /etc/matcha/apps/plausible/.env

$ nano /etc/matcha/apps/plausible/.env
DATABASE_URL=sqlite:///var/lib/plausible/db/plausible.db
SECRET_KEY_BASE=abc123...

$ matcha deploy plausible
Pulling image...            ✓
Starting plausible...       ✓
Registering with proxy...   ✓
Health check passed...      ✓

Done! https://analytics.example.com
```

**Day-to-day:**

```
$ matcha list
NAME         DOMAIN                    STATUS    IMAGE
plausible    analytics.example.com     running   plausible/analytics:latest
gitea        git.example.com           running   gitea:latest

$ matcha update plausible    # pull latest, zero-downtime redeploy
$ matcha status plausible    # detailed container info
$ matcha logs plausible      # view logs
$ matcha remove plausible    # stop, unregister, clean up
```

## On-Disk Structure

```
/etc/matcha/
├── matcha.json              # shared config (proxy image, network name)
├── apps/
│   ├── fusionaly/
│   │   ├── app.json         # image, domain, port, volumes, health path
│   │   └── .env             # secrets and env vars
│   ├── plausible/
│   │   ├── app.json
│   │   └── .env
│   └── gitea/
│       ├── app.json
│       └── .env
```

- `app.json` — machine-managed (written by `matcha add` or embedded library)
- `.env` — user-editable (secrets, custom env vars passed to container)

## Per-App Config (app.json)

```json
{
  "name": "plausible",
  "image": "plausible/analytics:latest",
  "domain": "analytics.example.com",
  "port": 8000,
  "health_path": "/api/health",
  "volumes": ["/data:/var/lib/plausible/db"],
  "backups": false
}
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `matcha setup` | Install Docker, create network, start shared proxy |
| `matcha add <name> --image --domain [--port] [--volume] [--health-path]` | Register a new app |
| `matcha deploy <name>` | Pull image, start container, register with proxy |
| `matcha update <name>` | Pull latest image, zero-downtime redeploy |
| `matcha remove <name>` | Stop container, unregister from proxy |
| `matcha list` | Show all apps and their status |
| `matcha status <name>` | Detailed info for one app |
| `matcha logs <name>` | View app logs |
| `matcha exec <name> <cmd>` | Run command in app container |
| `matcha backup <name>` | Manual SQLite backup |
| `matcha restore <name>` | List and restore from backups |
| `matcha migrate` | Migrate from old per-app proxy to shared proxy |

## Backups

Same as current single-app model, applied per-app. Each app with `backups: true` (or `--backups` flag) gets:
- Backup of `*.db` files from the app's data volume
- Retention: 7 daily, 14 weekly, 90 monthly
- Stored at `/etc/matcha/apps/{name}/backups/`

## Migration from Current Model

`matcha migrate` (or auto-detected on `install`) handles:

1. Detect old-style install (`/opt/{name}/.env` exists, `{name}-proxy` running)
2. Stop old per-app proxy (`fusionaly-proxy`)
3. Start shared `matcha-proxy` if not running
4. Move config from `/opt/{name}/` to `/etc/matcha/apps/{name}/`
5. Reconnect app container to `matcha-network`
6. Register app with shared proxy
7. Preserve volumes, secrets, domain — no data loss

## Breaking Changes from Current Model

| Before | After |
|--------|-------|
| `fusionaly-proxy` (per-app) | `matcha-proxy` (shared) |
| `fusionaly-network` (per-app) | `matcha-network` (shared) |
| `/opt/fusionaly/.env` | `/etc/matcha/apps/fusionaly/.env` + `app.json` |
| Ports 80/443 per-app proxy | Ports 80/443 shared proxy |

## What Stays the Same

- Go library API: `matcha.New(Config{...})`, `m.Install()`, `m.Update()`
- Zero-downtime deploys via kamal-proxy
- Automatic Let's Encrypt TLS
- SQLite backups with retention
- Self-updating manager binaries
- Per-app `.env` files for secrets
