# Feature: matcha update / update-all

## Goal
Pull latest images and redeploy with zero downtime.

## Behavior
- `matcha update <name>` — updates single app
- `matcha update-all` — updates all registered apps
- Checks for CLI self-update from GitHub releases first
- Pulls latest Docker images (skips if digest unchanged)
- Deploys with zero-downtime strategy
- Prunes unused images after deploy
- Creates backup before deploy if backups enabled

## Acceptance
- [ ] Self-updates matcha CLI before updating apps
- [ ] Pulls images with retry (3 attempts)
- [ ] Zero-downtime deploy
- [ ] Prunes old images
- [ ] `update-all` iterates all apps, continues on individual failure
