# Feature: matcha check

## Goal
Help solo devs secure their server by showing what to do and checking what's already done.

## Behavior
- `matcha check` runs on the server (same as other matcha commands)
- Prints a checklist of security checks
- Each item shows: status (done/not done), what it is, how to fix it
- Matcha checks the current state but never modifies anything — user does the work
- Exit code 0 if all checks pass, 1 if any fail
- After `matcha setup`, print a reminder: "Run `matcha check` to verify your server security"

## Checks

1. **SSH password auth disabled** — check `PasswordAuthentication no` in `/etc/ssh/sshd_config`
2. **SSH root login disabled** — check `PermitRootLogin no` in `/etc/ssh/sshd_config`
3. **Firewall active** — check `ufw status` is active
4. **Only necessary ports open** — check ufw allows 22, 80, 443 only (warn on others)
5. **Unattended upgrades enabled** — check `unattended-upgrades` package is installed
6. **Cloudflare proxy** — check if domain DNS resolves to CF IPs (optional, informational)
7. **Tailscale installed** — check if `tailscale` binary exists (optional, informational)

## Output

```
$ matcha check

Security check:

  [x] SSH password authentication disabled
  [ ] SSH root login disabled
      → Edit /etc/ssh/sshd_config: set PermitRootLogin no
      → Then: systemctl restart sshd
  [x] Firewall (ufw) active
  [!] Unexpected open port: 8080
      → Run: ufw delete allow 8080
  [x] Unattended security upgrades enabled
  [ ] Tailscale not installed (optional)
      → Install: curl -fsSL https://tailscale.com/install.sh | sh
  [ ] Cloudflare proxy not detected for myapp.example.com (optional)
      → See: https://github.com/karloscodes/matcha#recommended-cloudflare-proxy

Result: 3/5 required checks passed. 0/2 optional.
```

## Acceptance
- [ ] Checks SSH password auth config
- [ ] Checks SSH root login config
- [ ] Checks ufw is active and reports unexpected ports
- [ ] Checks unattended-upgrades is installed
- [ ] Checks Tailscale presence (optional, no fail)
- [ ] Checks Cloudflare proxy for configured domains (optional, no fail)
- [ ] Prints actionable fix instructions for each failing check
- [ ] Never modifies the system — read-only
- [ ] Exit code reflects required checks only
- [ ] `matcha setup` prints reminder to run `matcha check` after completion

## Notes
- Runs as root (same as other matcha commands)
- Only supports Ubuntu/Debian for now (that's what solos use on Hetzner/DO/Vultr)
- Optional checks show `[ ]` but don't affect exit code
- Keep it simple — no flags, no options, just `matcha check`
