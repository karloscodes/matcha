# Matcha

Deploy Docker apps with automatic SSL, zero-downtime updates, and SQLite backups. Powered by [kamal-proxy](https://github.com/basecamp/kamal-proxy).

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
curl -fsSL https://get.matcha.dev | sh

# Set up shared infrastructure
matcha setup

# Add and deploy apps
matcha add plausible --image plausible/analytics:latest --domain analytics.example.com --port 8000
nano /etc/matcha/apps/plausible/.env   # add DATABASE_URL, SECRET_KEY, etc.
matcha deploy plausible

matcha add gitea --image gitea:latest --domain git.example.com --port 3000 --volume /data:/data
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

For your own apps, embed Matcha to get a self-contained binary with install, update, and backup commands:

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
        Backups:        true,
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

Both modes use the same shared proxy and app registry. An app installed via `myapp install` shows up in `matcha list`.

## CLI commands

| Command | Description |
|---------|-------------|
| `matcha setup` | Install Docker, create network, start shared proxy |
| `matcha add <name> --image --domain [--port] [--volume] [--health-path] [--backups]` | Register a new app |
| `matcha deploy <name>` | Pull image and deploy |
| `matcha update <name>` | Pull latest image, zero-downtime redeploy |
| `matcha list` | Show all apps |
| `matcha status <name>` | Container details |
| `matcha logs <name>` | Stream app logs |
| `matcha exec <name> <cmd>` | Run command in container |
| `matcha remove <name>` | Stop and unregister |
| `matcha backup <name>` | Create SQLite backup |
| `matcha restore <name>` | Restore from backup |
| `matcha migrate <name>` | Migrate from old per-app layout |

## Config (Go library)

| Field | Default | Description |
|-------|---------|-------------|
| `Name` | *required* | App name (container name, env prefix, registry key) |
| `AppImage` | *required* | Docker image to deploy |
| `AppPort` | `8080` | Port your app listens on |
| `HealthPath` | `/up` | Health check endpoint (must return 200) |
| `Volumes` | `[]` | Custom volume mounts (`host:container` format) |
| `Backups` | `false` | SQLite backup with retention (7 daily, 14 weekly, 90 monthly) |
| `CronUpdates` | `false` | Daily 3 AM auto-update cron job |
| `ProxyImage` | `basecamp/kamal-proxy:latest` | kamal-proxy image |
| `InstallDir` | `/etc/matcha/apps/{Name}` | App config and data directory |
| `ManagerRepo` | `""` | GitHub repo for self-updating the binary |
| `ManagerVersion` | `""` | Current version (set via ldflags) |

## On-disk layout

```
/etc/matcha/
├── proxy-data/              # kamal-proxy TLS certs and state
├── apps/
│   ├── fusionaly/
│   │   ├── app.json         # image, domain, port, volumes
│   │   ├── .env             # secrets (user-editable)
│   │   ├── storage/         # app data volume → /app/storage
│   │   └── logs/            # app logs volume → /app/logs
│   └── plausible/
│       ├── app.json
│       ├── .env
│       ├── storage/
│       └── logs/
```

## Environment variables

Matcha auto-generates these for each container:

| Variable | Example |
|----------|---------|
| `{NAME}_DOMAIN` | `MYAPP_DOMAIN=app.example.com` |
| `{NAME}_PRIVATE_KEY` | `MYAPP_PRIVATE_KEY=abc123...` |
| `{NAME}_APP_PORT` | `MYAPP_APP_PORT=8080` |
| `{NAME}_ENV` | `MYAPP_ENV=production` |

Any additional variables in the app's `.env` file are also passed to the container.

## Volumes

Default mounts for every app:

- `/app/storage` — persistent data (SQLite, uploads)
- `/app/logs` — application logs

Custom volumes via `--volume` flag or `Volumes` config field.

## Migration from v0.x

If upgrading from the old per-app proxy layout (`/opt/{name}/`):

```bash
matcha migrate myapp
matcha deploy myapp
```

This moves config from `/opt/{name}/` to `/etc/matcha/apps/{name}/` and registers the app with the shared proxy.

## License

MIT
