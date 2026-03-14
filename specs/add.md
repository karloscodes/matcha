# Feature: matcha add

## Goal
Register a new app in matcha's config so it can be deployed.

## Behavior
- `matcha add <name> --image <img> --domain <domain> [--port N] [--health-path /path] [--volume /path] [--env KEY=VAL]`
- Saves app config to `/etc/matcha/config.yml`
- Generates a `PRIVATE_KEY` for the app
- Creates host directories for volumes
- Does not deploy — just registers

## Acceptance
- [ ] Requires `--image` and `--domain`
- [ ] Defaults: port 8080, health-path `/up`
- [ ] Supports multiple `--volume` and `--env` flags
- [ ] Saves to YAML config
- [ ] Creates volume directories under `/var/matcha/{name}/`
- [ ] Prints confirmation and next step: `matcha deploy <name>`
