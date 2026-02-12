# Matcha — Containerized App Deployment Library

Go library for deploying containerized applications with zero-downtime updates.

## What It Does

- Docker installation & orchestration
- Blue-green deployments with health checks
- Caddy reverse proxy with auto-SSL
- Cron-based auto-updates
- SQLite backup/restore

## Usage

```go
import "github.com/karloscodes/matcha"

func main() {
    m := matcha.New(matcha.Config{
        Name:      "fusionaly",
        AppImage:  "karloscodes/fusionaly:latest",
        BlueGreen: true,
        Backups:   true,
    })

    switch os.Args[1] {
    case "install":
        m.Install()
    case "update":
        m.Update()
    case "reload":
        m.Reload()
    case "restore-db":
        m.RestoreDB()
    case "status":
        m.Status()

    // Project-specific commands (not Matcha)
    case "upgrade":
        handleUpgradeToPro(m)
    }
}
```

## Config Struct

```go
type Config struct {
    // Required
    Name       string  // "fusionaly" → env prefix FUSIONALY_, container names, etc.
    AppImage   string  // "karloscodes/fusionaly:latest"

    // Optional with defaults
    InstallDir  string  // default: /opt/{Name}
    BinaryPath  string  // default: /usr/local/bin/{Name}
    CaddyImage  string  // default: caddy:2-alpine
    HealthPath  string  // default: /_health
    AppPort     int     // default: 8080

    // Feature flags (all default false)
    BlueGreen   bool    // dual containers, zero-downtime switch
    CronUpdates bool    // daily 3 AM auto-update cron job
    Backups     bool    // SQLite backup with retention policy
}
```

## Environment Variables

Matcha manages these in `.env`:

| Variable | Owner | Source |
|----------|-------|--------|
| `{NAME}_DOMAIN` | User | Prompted during install |
| `{NAME}_PRIVATE_KEY` | Matcha | Generated once, preserved forever |

Port (8080) is hardcoded in container, not in .env.

### .env Handling

- First install: create with DOMAIN + PRIVATE_KEY
- Re-install: update DOMAIN, preserve PRIVATE_KEY, **leave all other lines untouched**
- User can add any vars they want — Matcha won't touch them

## Methods

| Method | What it does |
|--------|--------------|
| `Install()` | Check root, install Docker, prompt domain, generate private key, deploy containers, setup cron |
| `Update()` | Pull latest image, blue-green deploy, upgrade binary, prune old images |
| `Reload()` | Restart containers with current .env (no image pull) |
| `RestoreDB()` | List backups, prompt selection, restore SQLite |
| `Status()` | Show running containers, versions, health |

## Package Structure

```
github.com/karloscodes/matcha/
├── matcha.go           # New(), Config struct, public methods
├── installer.go        # Install orchestration
├── docker.go           # Container operations, blue-green
├── caddy.go            # Reverse proxy, SSL
├── config.go           # .env handling, user prompts
├── cron.go             # Auto-update scheduling
├── backup.go           # Database backup/restore
├── requirements.go     # System checks (root, ports)
├── logging.go          # Logger
└── templates/
    └── Caddyfile.tmpl
```

## Docker Naming Convention

For `Name: "fusionaly"`:
- Network: `fusionaly-network`
- App containers: `fusionaly-app-1`, `fusionaly-app-2` (blue-green)
- Caddy: `fusionaly-caddy`

## Blue-Green Deployment

When `BlueGreen: true`:
1. Deploy new container (app-2 if app-1 is running)
2. Health check at `http://localhost:8080{HealthPath}`
3. Update Caddy to point to new container
4. Stop old container
5. Prune unused images

When `BlueGreen: false`:
- Single container, brief downtime on update

## Caddyfile Template

```
{{.Domain}} {
    tls {{.TLSConfig}}

    reverse_proxy {{.ActiveContainer}}:8080 {
        health_uri /_health
        health_interval 5s
    }

    log {
        output file /data/logs/caddy.log
        format json
    }

    encode zstd gzip
}
```

## Project Integration

Each project imports Matcha and:
1. Defines its Config
2. Wires commands to CLI args
3. Adds project-specific commands as needed

Matcha is a library, not a framework. No hooks, no plugins — just call the methods.
