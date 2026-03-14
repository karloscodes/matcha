# Feature: matcha logs / exec

## Goal
Interact with running app containers.

## Behavior
- `matcha logs <name>` — streams last 100 lines + follows
- `matcha exec <name> <cmd...>` — runs command in active container
- Both find the active container (base or -next)

## Acceptance
- [ ] `logs` streams with `--tail 100 -f`
- [ ] `exec` passes all remaining args to container
- [ ] Both work regardless of which container variant is active
