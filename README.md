# Matcha

Deploy Docker apps to your own Linux server. You get automatic HTTPS, zero-downtime updates, and nightly auto-updates. There is no control plane and no YAML to write by hand.

Matcha runs one shared [kamal-proxy](https://github.com/basecamp/kamal-proxy) in front of all your apps. You can use it two ways:

- **As a CLI.** Deploy any Docker image with `matcha add` and `matcha deploy`.
- **As a Go library.** Ship your app as a single binary that installs, updates, and manages itself (`myapp install`).

## Requirements

- Linux, amd64 or arm64. Debian or Ubuntu is best tested.
- Root access. Every command writes to `/etc` and `/var`.
- Ports 80 and 443 free and open to the internet.
- A domain with an A record that points to the server.

Matcha installs Docker for you if it is missing.

## Quick start

```bash
# 1. Install the matcha binary
curl -fsSL https://raw.githubusercontent.com/karloscodes/matcha/master/install.sh | sudo sh

# 2. Install Docker, create the network, start the proxy, add the nightly update cron
sudo matcha setup

# 3. Register an app
sudo matcha add whoami --image traefik/whoami --domain whoami.example.com --port 80 --health-path /

# 4. Deploy it
sudo matcha deploy whoami
```

Point `whoami.example.com` to the server with an A record. kamal-proxy gets a Let's Encrypt certificate on the first HTTPS request after DNS resolves. Then open `https://whoami.example.com`.

Check the server security when you are done:

```bash
sudo matcha check
```

## How it works

```
Internet → matcha-proxy (ports 80/443, TLS) → app containers (internal ports)
                 ↓
         routes by hostname
         ↓              ↓              ↓
    fusionaly:8080  plausible:8000  gitea:3000
```

- One kamal-proxy container (`matcha-proxy`) owns ports 80 and 443. It routes each request by hostname.
- Each app runs as one container on the `matcha-network` Docker network. Apps do not publish ports on the host.
- kamal-proxy gets and renews TLS certificates from Let's Encrypt.

### Zero-downtime deploys

Each deploy swaps between two container names: `{name}` and `{name}-next`.

1. Matcha starts the new container next to the old one.
2. kamal-proxy calls the health path on the new container until it returns `200`.
3. kamal-proxy moves traffic to the new container. Matcha then removes the old one.
4. If the health check fails before the timeout (30s by default), Matcha removes the new container. The old container keeps serving traffic.

This works with one container per app. Both containers mount the same volumes for a few seconds during the swap.

## CLI

### Commands

| Command | What it does |
|---|---|
| `matcha setup` | Install Docker, create `matcha-network`, start `matcha-proxy`, install the nightly update cron |
| `matcha add <name> [flags]` | Register an app in `/etc/matcha/config.yml`. Does not start it |
| `matcha deploy <name>` | Start or restart the app with the current config. Does not pull a new image |
| `matcha update <name>` | Update the matcha binary, pull the latest image, redeploy |
| `matcha update-all` | Same as `update`, for every registered app |
| `matcha list` (`ls`) | Show all apps with image, domain, and port |
| `matcha status <name>` | Show proxy and app container state, image, and start time |
| `matcha logs <name>` | Follow the app logs (last 100 lines, then live) |
| `matcha exec <name> <cmd...>` | Run a command in the app container |
| `matcha remove <name>` (`rm`) | Remove the app from the proxy, stop it, delete its config. Keeps its data |
| `matcha check` | Check SSH, firewall, security updates, Cloudflare, and Tailscale. Changes nothing |
| `matcha version` | Print the version |

### `matcha add` flags

| Flag | Default | Description |
|---|---|---|
| `--image` | *required* | Docker image, for example `plausible/analytics:latest` |
| `--domain` | *required* | Public hostname. Use `app.localhost` for local tests without TLS |
| `--port` | `8080` | Port the app listens on inside the container |
| `--health-path` | `/up` | Path that returns `200` when the app is ready |
| `--volume` | none | Container path to persist. Repeat for more than one |
| `--env KEY=VALUE` | none | Environment variable. Repeat for more than one |

> **The health path matters.** Most third-party images do not serve `/up`. If the deploy fails with a health check error, set `--health-path /` or the path the image documents.

### Examples

```bash
sudo matcha add plausible \
  --image plausible/analytics:latest \
  --domain analytics.example.com \
  --port 8000 \
  --health-path /api/health \
  --env SECRET_KEY_BASE=$(openssl rand -hex 64)

sudo matcha add gitea --image gitea/gitea:latest --domain git.example.com --port 3000 --volume /data --health-path /

sudo matcha deploy plausible
sudo matcha deploy gitea
```

### Change an app

`matcha add` has no edit mode. To change the image, port, health path, or env vars, edit the config file and deploy again:

```bash
sudo $EDITOR /etc/matcha/config.yml
sudo matcha deploy plausible
```

## Config file

Matcha keeps all app config in one file: `/etc/matcha/config.yml` (mode `0600`, because it holds secrets).

```yaml
apps:
  plausible:
    image: plausible/analytics:latest
    domain: analytics.example.com
    port: 8000
    health_path: /api/health
    health_timeout: 90          # seconds; optional, default 30
    volumes:
      - /var/lib/plausible
    env:
      PRIVATE_KEY: 3f9a...      # generated by matcha
      SECRET_KEY_BASE: abc123
```

Use `health_timeout` for apps that take more than 30 seconds to boot. There is no CLI flag for it.

## Data and volumes

Matcha maps each volume to `/var/matcha/{name}/{last path segment}` on the host:

| `--volume` | Host path |
|---|---|
| `/app/storage` | `/var/matcha/myapp/storage` |
| `/data` | `/var/matcha/myapp/data` |
| `/var/lib/plausible` | `/var/matcha/myapp/plausible` |

Two volumes with the same last segment (for example `/a/data` and `/b/data`) share one host directory. Avoid that.

```
/etc/matcha/config.yml            # all apps, env vars included
/etc/cron.d/matcha-update         # nightly update-all (created by setup)
/var/log/matcha-update.log        # output of the nightly update
/var/matcha/
├── {name}/                       # one directory per app, one subdir per volume
│   └── backups/                  # SQLite backups (library, Backups: true)
└── proxy/                        # kamal-proxy certificates and state
```

`matcha remove` never deletes `/var/matcha/{name}`. Delete it yourself when you no longer need the data.

## Environment variables

Matcha sets these in every app container. `{NAME}` is the app name in upper case.

| Variable | Example | Notes |
|---|---|---|
| `PRIVATE_KEY` | `3f9a…` (64 hex chars) | Random secret, generated once at `add` or `install` |
| `{NAME}_PRIVATE_KEY` | same value | Kept for older apps |
| `{NAME}_DOMAIN` | `PLAUSIBLE_DOMAIN=analytics.example.com` | |
| `{NAME}_APP_PORT` | `PLAUSIBLE_APP_PORT=8000` | |
| `{NAME}_ENV` | `PLAUSIBLE_ENV=production` | |
| `MATCHA_MANAGER_VERSION` | `v1.4.2` | Library only, when `ManagerVersion` is set |

Use `PRIVATE_KEY` to sign sessions or tokens, or ignore it. Add your own variables with `--env` or in the `env:` block of the config file.

Each app container has a 512 MB memory limit and the restart policy `unless-stopped`.

## Updates

### Nightly app updates

`matcha setup` installs a cron job that runs `matcha update-all` every day at 3 AM. The output goes to `/var/log/matcha-update.log`. For each app, Matcha:

1. Runs `docker pull`. If the tag has not changed, Docker only checks the digest and downloads nothing.
2. Deploys with the zero-downtime swap above.
3. Removes dangling images with `docker image prune -f`.

A failure in one app does not stop the updates of the other apps.

Use tags that move, such as `:latest` or `:1`, if you want nightly updates. A fixed tag such as `:1.4.2` never changes.

### Self-update of the matcha binary

`matcha update` and `matcha update-all` first check [the latest release](https://github.com/karloscodes/matcha/releases). If it is newer, Matcha downloads the binary, verifies its SHA256 against `checksums.txt`, replaces `/usr/local/bin/matcha`, and restarts with the new binary.

## Go library: a self-deploying binary

Import Matcha to give your app a single binary that does the whole server setup:

```bash
go get github.com/karloscodes/matcha
```

```go
package main

import (
    "fmt"
    "os"

    "github.com/karloscodes/matcha"
)

var version = "dev" // set with -ldflags "-X main.version=1.2.3"

func main() {
    m := matcha.New(matcha.Config{
        Name:     "myapp",
        AppImage: "ghcr.io/user/myapp:latest",

        AppPort:        8080,
        HealthPath:     "/up",
        Volumes:        []string{"/app/storage"},
        CronUpdates:    true,
        Backups:        true,
        ManagerRepo:    "user/myapp",
        ManagerVersion: version,
    })

    if len(os.Args) < 2 {
        fmt.Println("Usage: myapp <install|update|status|logs|exec|backup|restore>")
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
    case "logs":
        err = m.Logs()
    case "exec":
        err = m.Exec(os.Args[2:]...)
    case "backup":
        _, err = m.BackupDB()
    case "restore":
        err = m.RestoreDB()
    }

    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

`myapp install` asks for the domain, checks DNS, installs Docker, starts the proxy, deploys the app, installs the cron job, and copies itself to `/usr/local/bin/myapp`.

Apps installed this way use the same proxy and config file as the CLI. They show up in `matcha list`.

### Methods

| Method | What it does |
|---|---|
| `Install()` | Interactive first install. Needs root |
| `Update()` | Self-update the binary, then pull and redeploy every app in the config file |
| `Deploy()` | Redeploy with the current config. No pull |
| `Reload()` | Same as `Deploy()`, with progress output |
| `Status()` | Print proxy and app state |
| `Logs()` | Follow the app logs |
| `Exec(args...)` | Run a command in the app container |
| `BackupDB()` | Back up the first `*.db` file in the data directory. Returns the backup path |
| `RestoreDB()` | List backups, ask which one to restore. Needs `Backups: true` |
| `SetImage(img)` + `SaveImage()` | Change the image and save it to the config file |
| `GetDomain()` | Read the app domain |

### Config

| Field | Default | Description |
|---|---|---|
| `Name` | *required* | App name. Used for the container name, the env prefix, and the data directory |
| `AppImage` | *required* | Docker image to deploy |
| `AppPort` | `8080` | Port the app listens on |
| `HealthPath` | `/up` | Path that returns `200` when the app is ready |
| `HealthTimeout` | `30` | Seconds the new container has to become healthy |
| `Volumes` | none | Container paths to persist, for example `/app/storage` |
| `CronUpdates` | `false` | Install `/etc/cron.d/{name}-update` to run `{name} update` daily at 3 AM |
| `Backups` | `false` | Back up the SQLite database before each deploy. Keeps the last 3 |
| `BinaryPath` | `/usr/local/bin/{Name}` | Where `Install()` copies the binary |
| `ProxyImage` | `basecamp/kamal-proxy:latest` | kamal-proxy image |
| `ManagerRepo` | none | GitHub repo for self-update, for example `user/myapp` |
| `ManagerVersion` | none | Current version. Self-update is off when empty. `dev` always updates to the latest release |

### Backups

With `Backups: true`, Matcha backs up the database before each deploy:

- It uses the first `*.db` file in `/var/matcha/{name}/` or `/var/matcha/{name}/storage/`.
- It writes `backups/backup_YYYYMMDD_HHMMSS.db`, runs `PRAGMA integrity_check`, and keeps the last 3.
- It needs `sqlite3` on the host (`apt-get install sqlite3`). Without it, the backup fails and the deploy continues.

### Release setup for self-update

1. Set `ManagerRepo` and `ManagerVersion`, and inject the version at build time:

   ```bash
   go build -ldflags "-X main.version=1.2.3" -o myapp ./cmd/myapp/
   ```

2. Publish GitHub releases with semver tags (`v1.2.3`) and these assets:
   - `myapp-linux-amd64` and `myapp-linux-arm64` (raw binaries, not archives)
   - `checksums.txt` with lines in the format `<sha256>  <filename>`

[`.goreleaser.yml`](.goreleaser.yml) in this repo produces exactly this layout.

## Security

### `matcha check`

Run `sudo matcha check` after setup. It checks the server and prints how to fix each problem. It never changes the system.

| Check | Required |
|---|---|
| SSH password login disabled | yes |
| SSH root login disabled | yes |
| `ufw` firewall active | yes |
| Only ports 22, 80, 443 open | yes |
| Unattended security upgrades installed | yes |
| Tailscale installed | optional |
| Cloudflare proxy in front of each app domain | optional |

The exit code is `1` if a required check fails.

### Recommended: Cloudflare proxy

Put [Cloudflare](https://www.cloudflare.com/) in front of the server to hide its IP address. It is free and needs no change in Matcha.

1. Add your domain to Cloudflare and point the DNS records to the server.
2. Turn on the proxy (orange cloud) for the records.
3. Set SSL/TLS mode to **Full (Strict)**.

Cloudflare terminates TLS at the edge. kamal-proxy still serves its own Let's Encrypt certificate to Cloudflare, so the full path stays encrypted.

### Real client IP

Matcha starts every app with kamal-proxy `--forward-headers`. Without it, kamal-proxy drops forwarded headers when TLS is on, and the app sees only the proxy IP.

Read the client IP from these headers, in this order:

1. `CF-Connecting-IP` (behind Cloudflare)
2. `X-Forwarded-For` (first address)
3. `X-Real-IP`

Only trust these headers when the request comes from the proxy. Anyone can send them to the proxy.

## Troubleshooting

| Problem | What to do |
|---|---|
| Deploy fails with a health check error | Check the path with `docker exec <name> wget -qO- localhost:<port><path>`. Set the right `health_path`. Raise `health_timeout` for slow boots |
| `ports [80 443] are not available` | Another web server runs on the host. Stop it: `systemctl disable --now nginx` (or apache2, caddy) |
| No HTTPS certificate | Check that the A record points to this server: `dig +short app.example.com`. Behind Cloudflare, use Full (Strict), not Flexible |
| App runs but shows the wrong site | Check the domain in `/etc/matcha/config.yml`, then `matcha deploy <name>` |
| See proxy logs | `docker logs -f matcha-proxy` |
| Interactive shell in the app | `docker exec -it $(docker ps -qf name=^myapp) sh` |
| Nightly update did not run | Check `/var/log/matcha-update.log` and `/etc/cron.d/matcha-update` |

## Development

```bash
go test -short ./...   # unit tests; drop -short to also run the Docker integration tests
go build -o matcha ./cmd/matcha
```

Feature specs live in [`specs/`](specs/). GoReleaser builds a release when you push a `v*` tag.

## License

MIT
