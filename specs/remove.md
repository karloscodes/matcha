# Feature: matcha remove

## Goal
Stop an app and remove it from matcha's config.

## Behavior
- `matcha remove <name>` (alias: `matcha rm`)
- Removes app from kamal-proxy routing
- Stops and removes both containers (`{name}` and `{name}-next`)
- Removes app from YAML config
- Does NOT delete data directory — user must do that manually

## Acceptance
- [ ] Unregisters from proxy
- [ ] Stops both container variants
- [ ] Removes from config YAML
- [ ] Preserves data directory
- [ ] Prints note about manual data cleanup
