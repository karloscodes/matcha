# Feature: matcha setup

## Goal
Install shared infrastructure so the server is ready to host apps.

## Behavior
- Installs Docker if not present
- Creates `matcha-network` Docker network
- Starts `matcha-proxy` container (kamal-proxy) on ports 80/443
- Sets up daily cron job for `matcha update-all` at 3 AM
- Prints reminder to run `matcha check` after completion

## Acceptance
- [ ] Docker installed and running
- [ ] `matcha-network` created
- [ ] `matcha-proxy` running on ports 80/443
- [ ] Cron job at `/etc/cron.d/matcha-update`
- [ ] Prints next step: `matcha add`
