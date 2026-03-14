# Feature: matcha status

## Goal
Show the current state of an app's deployment.

## Behavior
- `matcha status <name>`
- Shows proxy and app container status (running/stopped)
- Shows image, start time, domain URL

## Acceptance
- [ ] Shows container status with color
- [ ] Shows HTTPS URL for domain
- [ ] Works with either base or -next container
