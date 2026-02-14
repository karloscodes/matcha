# Matcha

Go library for deploying containerized applications with automatic SSL, zero-downtime updates, and SQLite backup management.

**This is not a generic Docker orchestration tool.** Matcha is purpose-built for deploying a specific stack: Caddy reverse proxy + Docker app container + SQLite database. It powers [Fusionaly](https://fusionaly.com), [Formlander](https://formlander.com), and [Lognorth](https://lognorth.com).

## What it does

- Deploys your app behind Caddy with automatic Let's Encrypt SSL
- Blue-green deployments for zero downtime
- SQLite backups with retention policies
- Self-updating manager binaries
- DNS validation during setup

## Usage

Embed matcha in your project's CLI:

```go
package main

import "github.com/karloscodes/matcha"

func main() {
    m := matcha.New(matcha.Config{
        Name:     "myapp",
        AppImage: "ghcr.io/user/myapp:latest",
        BlueGreen: true,
        Backups:   true,
    })

    // m.Install(), m.Update(), m.Status(), etc.
}
```

## License

MIT
