# Matcha

Go library for deploying containerized applications with automatic SSL, zero-downtime updates, and SQLite backup management.

**This is not a generic Docker orchestration tool.** Matcha is purpose-built for deploying a single Docker app per server with [kamal-proxy](https://github.com/basecamp/kamal-proxy) handling reverse proxy, TLS, and zero-downtime deploys. It powers [Fusionaly](https://fusionaly.com), [Formlander](https://formlander.com), and [Lognorth](https://lognorth.com).

## How it works

Matcha runs two Docker containers on your server:

```
Internet → kamal-proxy (ports 80/443, TLS) → your-app (internal port)
```

**kamal-proxy** handles:
- Automatic Let's Encrypt SSL certificates
- Zero-downtime deploys (health checks new container, buffers requests during switch, drains old connections)
- Host-based routing

**Matcha** handles:
- Installing Docker and kamal-proxy on a fresh server
- Pulling and running your app container
- Registering your app with kamal-proxy
- SQLite backups with retention policies (daily/weekly/monthly)
- Self-updating manager binaries via GitHub releases
- Automatic daily updates via cron

### Deploy flow

```
Install:  check system → install Docker → start kamal-proxy → start app → register with proxy
Update:   pull new image → start new container → kamal-proxy health checks → switch traffic → done
```

On update, kamal-proxy health-checks the new container before switching. If the new container is unhealthy, traffic stays on the old one and the deploy fails safely.

## Usage

Embed matcha in your project's CLI:

```go
package main

import (
    "fmt"
    "os"

    "github.com/karloscodes/matcha"
)

var version = "dev" // set via ldflags at build time

func main() {
    m := matcha.New(matcha.Config{
        Name:     "myapp",
        AppImage: "ghcr.io/user/myapp:latest",

        // Optional
        AppPort:    8080,                // port your app listens on (default: 8080)
        HealthPath: "/up",               // health check endpoint (default: /up)
        Backups:    true,                // SQLite backup with retention
        CronUpdates: true,               // daily 3 AM auto-update
        ManagerRepo:    "user/myapp",    // GitHub repo for self-updates
        ManagerVersion: version,         // current version
    })

    if len(os.Args) < 2 {
        fmt.Println("Usage: myapp <install|update|reload|status|restore-db>")
        os.Exit(1)
    }

    var err error
    switch os.Args[1] {
    case "install":
        err = m.Install()
    case "update":
        err = m.Update()
    case "reload":
        err = m.Reload()
    case "status":
        err = m.Status()
    case "restore-db":
        err = m.RestoreDB()
    case "exec":
        err = m.Exec(os.Args[2:]...)
    }

    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

Build and distribute this binary. On your server:

```bash
# First time: installs Docker, kamal-proxy, deploys your app
myapp install

# Pulls latest image, zero-downtime redeploy
myapp update

# Check what's running
myapp status

# Run a command inside the app container
myapp exec sh
```

## Config

| Field | Default | Description |
|-------|---------|-------------|
| `Name` | *required* | App name. Used for container names, env prefix, install directory. |
| `AppImage` | *required* | Docker image to deploy (e.g., `ghcr.io/user/myapp:latest`). |
| `AppPort` | `8080` | Port your app listens on inside the container. |
| `HealthPath` | `/up` | Endpoint kamal-proxy checks before switching traffic. Must return 200. |
| `ProxyImage` | `basecamp/kamal-proxy:latest` | kamal-proxy Docker image. |
| `InstallDir` | `/opt/{Name}` | Where config and data are stored on the server. |
| `Backups` | `false` | Enable SQLite backup with retention (7 daily, 14 weekly, 90 monthly). |
| `CronUpdates` | `false` | Set up a daily 3 AM cron job that runs `update`. |
| `ManagerRepo` | `""` | GitHub repo (e.g., `user/myapp`) for self-updating the manager binary. |
| `ManagerVersion` | `""` | Current version string (set via `-ldflags "-X main.version=v1.0.0"`). |

## What your app needs

1. **A Docker image** pushed to a registry (Docker Hub, GHCR, etc.)
2. **A health endpoint** (default `/up`) that returns HTTP 200 when ready
3. That's it

## Environment variables

Matcha passes these to your container:

| Variable | Example | Description |
|----------|---------|-------------|
| `{NAME}_DOMAIN` | `MYAPP_DOMAIN=app.example.com` | Configured domain |
| `{NAME}_PRIVATE_KEY` | `MYAPP_PRIVATE_KEY=abc123...` | Generated secret key |
| `{NAME}_APP_PORT` | `MYAPP_APP_PORT=8080` | Port config |
| `{NAME}_ENV` | `MYAPP_ENV=production` | Always "production" |

## Volumes

Matcha mounts two volumes into your container:

- `/app/storage` — persistent data (SQLite databases, uploads, etc.)
- `/app/logs` — application logs

These are stored on the host at `{InstallDir}/storage/` and `{InstallDir}/logs/`.

## License

MIT
