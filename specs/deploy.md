# Feature: matcha deploy

## Goal
Deploy a registered app with zero downtime.

## Behavior
- `matcha deploy <name>`
- Loads app config from YAML
- Creates network, ensures proxy is running
- Starts new container with alternate name (`{name}` ↔ `{name}-next`)
- Registers with kamal-proxy (health-checks the new container)
- Removes old container only after proxy switches traffic
- Prints DNS instructions

## Acceptance
- [ ] Fails if app not registered
- [ ] Zero-downtime: old container serves until new is healthy
- [ ] Container gets env vars: PRIVATE_KEY, {PREFIX}_DOMAIN, {PREFIX}_APP_PORT, {PREFIX}_ENV
- [ ] Volumes mounted correctly
- [ ] Prints DNS setup instructions after deploy
