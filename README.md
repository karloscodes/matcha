# Matcha

Deploy Docker apps with automatic SSL and zero-downtime updates. Powered by [kamal-proxy](https://github.com/basecamp/kamal-proxy).

## How it works

```
Internet → matcha-proxy (ports 80/443, TLS) → app containers (internal ports)
                 ↓
         routes by hostname
         ↓              ↓              ↓
    fusionaly:8080  plausible:8000  gitea:3000
```

One shared kamal-proxy routes traffic by hostname. Each app runs as a separate Docker container on `matcha-network`. TLS is automatic via Let's Encrypt.

On deploy, kamal-proxy health-checks the new container before switching traffic. If unhealthy, traffic stays on the old container. Zero downtime, even with a single replica.

## Two ways to use Matcha

### 1. Standalone CLI — deploy any Docker image

```bash
# Install matcha on your server
curl -fsSL https://raw.githubusercontent.com/karloscodes/matcha/main/install.sh | sh

# Set up shared infrastructure
matcha setup

# Add and deploy apps
matcha add plausible --image plausible/analytics:latest --domain analytics.example.com --port 8000 --env SECRET_KEY=abc123
matcha deploy plausible

matcha add gitea --image gitea:latest --domain git.example.com --port 3000 --volume /data
matcha deploy gitea

# Day-to-day
matcha list                    # show all apps
matcha update plausible        # pull latest, zero-downtime redeploy
matcha status plausible        # container details
matcha logs plausible          # stream logs
matcha exec plausible sh       # shell into container
matcha remove plausible        # stop and unregister
```

### 2. Embedded Go library — build your own installer

For your own apps, embed Matcha to get a self-contained binary with install and update commands:

```go
package main

import (
    "fmt"
    "os"

    "github.com/karloscodes/matcha"
)

var version = "dev"

func main() {
    m := matcha.New(matcha.Config{
        Name:     "myapp",
        AppImage: "ghcr.io/user/myapp:latest",

        // Optional
        AppPort:        8080,
        HealthPath:     "/up",
        Volumes:        []string{"/app/storage"},
        CronUpdates:    true,
        ManagerRepo:    "user/myapp",
        ManagerVersion: version,
    })

    if len(os.Args) < 2 {
        fmt.Println("Usage: myapp <install|update|status|exec>")
        os.Exit(1)
    }

    var err error
    switch os.Args[1] {
    case "install":
        err = m.Install()
    case "update":
        err = m.Update()
    case "status":
        err = m.Status()
    case "exec":
        err = m.Exec(os.Args[2:]...)
    }

    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

```bash
myapp install    # Docker + proxy + app, all in one command
myapp update     # pull latest, zero-downtime redeploy
```

Both modes use the same shared proxy and config. An app installed via `myapp install` shows up in `matcha list`.

## Updates

Set `CronUpdates: true` (Go library) or use a cron job to run `matcha update <name>` nightly. Here's what happens:

1. `docker pull` checks the remote image digest — if the `:latest` tag hasn't changed, no download happens
2. If there's a new image, the new container starts and kamal-proxy health-checks it
3. Once healthy, traffic switches to the new container. The old one is removed
4. If any `.db` files exist in the data dir, a pre-deploy snapshot is taken automatically via `sqlite3 .backup`

Updates are cheap when nothing changed — just a digest check.

## CLI commands

| Command | Description |
|---------|-------------|
| `matcha setup` | Install Docker, create network, start shared proxy |
| `matcha add <name> --image --domain [--port] [--volume] [--health-path] [--env KEY=VAL]` | Register a new app |
| `matcha deploy <name>` | Pull image and deploy |
| `matcha update <name>` | Pull latest image, zero-downtime redeploy |
| `matcha list` | Show all apps |
| `matcha status <name>` | Container details |
| `matcha logs <name>` | Stream app logs |
| `matcha exec <name> <cmd>` | Run command in container |
| `matcha remove <name>` | Stop and unregister |
| `matcha migrate <name>` | Migrate from old per-app layout |

## Config (Go library)

| Field | Default | Description |
|-------|---------|-------------|
| `Name` | *required* | App name (container name, env prefix) |
| `AppImage` | *required* | Docker image to deploy |
| `AppPort` | `8080` | Port your app listens on |
| `HealthPath` | `/up` | Health check endpoint (must return 200) |
| `Volumes` | `[]` | Container paths to mount (e.g., `/app/storage`) |
| `CronUpdates` | `false` | Daily 3 AM auto-update cron job |
| `ProxyImage` | `basecamp/kamal-proxy:latest` | kamal-proxy image |
| `ManagerRepo` | `""` | GitHub repo for self-updating the binary |
| `ManagerVersion` | `""` | Current version (set via ldflags) |

## On-disk layout

```
/etc/matcha/config.yml          # all app config + env vars (single file)

/var/matcha/
├── fusionaly/
│   └── storage/                # volume data → /app/storage
├── plausible/
│   └── data/                   # volume data → /data
└── proxy/                      # kamal-proxy TLS certs and state
```

Volumes are auto-resolved from container paths: `/app/storage` becomes `/var/matcha/{name}/storage:/app/storage`.

## Environment variables

Matcha auto-generates these for each container:

| Variable | Example |
|----------|---------|
| `{NAME}_DOMAIN` | `MYAPP_DOMAIN=app.example.com` |
| `{NAME}_APP_PORT` | `MYAPP_APP_PORT=8080` |
| `{NAME}_ENV` | `MYAPP_ENV=production` |

A `PRIVATE_KEY` is generated on first install. Additional env vars can be set via `--env` flag or directly in `/etc/matcha/config.yml`.

## Migration from old layouts

If upgrading from the old per-app layout (`/etc/matcha/apps/{name}/` or `/opt/{name}/`):

```bash
# Auto-migrates on first update
myapp update

# Or explicitly
matcha migrate myapp
```

Config and env vars are moved to `/etc/matcha/config.yml`, data to `/var/matcha/{name}/`.

## License

MIT
