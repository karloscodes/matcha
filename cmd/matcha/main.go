package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/karloscodes/matcha"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "setup":
		cmdSetup()
	case "add":
		cmdAdd()
	case "deploy":
		cmdDeploy()
	case "update":
		cmdUpdate()
	case "update-all":
		cmdUpdateAll()
	case "list", "ls":
		cmdList()
	case "status":
		cmdStatus()
	case "remove", "rm":
		cmdRemove()
	case "exec":
		cmdExec()
	case "logs":
		cmdLogs()
	case "check":
		cmdCheck()
	case "version":
		fmt.Println("matcha " + version)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: matcha <command> [args]

Commands:
  setup              Set up shared infrastructure (Docker, network, proxy)
  add <name>         Register a new app
  deploy <name>      Deploy an app
  update <name>      Pull latest image and redeploy
  update-all         Update all registered apps
  list (ls)          List all registered apps
  status <name>      Show app status
  remove (rm) <name> Remove an app
  exec <name> <cmd>  Run a command in the app container
  logs <name>        Stream app logs
  check              Check server security
  version            Print version`)
}

func extractSubdomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return "@"
	}
	return parts[0]
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

func requireAppName(cmd string) string {
	if len(os.Args) < 3 {
		fatal(fmt.Errorf("%s requires an app name. Usage: matcha %s <name>", cmd, cmd))
	}
	return os.Args[2]
}

func matchaFromConfig(name string) *matcha.Matcha {
	app, err := matcha.LoadApp(name)
	if err != nil {
		// App not in YAML yet — return minimal config.
		// loadConfig() in Update/Deploy will try auto-migration from old layouts.
		return matcha.New(matcha.Config{Name: name})
	}
	return matcha.NewFromApp(name, app, matcha.Config{})
}

func cmdSetup() {
	if err := matcha.Setup(); err != nil {
		fatal(err)
	}
	setupCron()
	fmt.Println("Matcha is ready. Add your first app with 'matcha add'.")
	fmt.Println()
	fmt.Println("Run 'matcha check' to verify your server security.")
}

func setupCron() {
	content := "# Matcha auto-update (all apps)\n0 3 * * * root /usr/local/bin/matcha update-all >> /var/log/matcha-update.log 2>&1\n"
	if err := os.WriteFile("/etc/cron.d/matcha-update", []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not set up cron: %v\n", err)
	}
}

func cmdAdd() {
	if len(os.Args) < 3 {
		fatal(fmt.Errorf("add requires an app name. Usage: matcha add <name> --image <img> --domain <domain> [options]"))
	}
	name := os.Args[2]

	var image, domain, healthPath string
	port := 8080
	var volumes []string
	env := make(map[string]string)

	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--image":
			i++
			if i >= len(args) {
				fatal(fmt.Errorf("--image requires a value"))
			}
			image = args[i]
		case "--domain":
			i++
			if i >= len(args) {
				fatal(fmt.Errorf("--domain requires a value"))
			}
			domain = args[i]
		case "--port":
			i++
			if i >= len(args) {
				fatal(fmt.Errorf("--port requires a value"))
			}
			p, err := strconv.Atoi(args[i])
			if err != nil {
				fatal(fmt.Errorf("invalid port: %s", args[i]))
			}
			port = p
		case "--health-path":
			i++
			if i >= len(args) {
				fatal(fmt.Errorf("--health-path requires a value"))
			}
			healthPath = args[i]
		case "--volume":
			i++
			if i >= len(args) {
				fatal(fmt.Errorf("--volume requires a value"))
			}
			volumes = append(volumes, args[i])
		case "--env":
			i++
			if i >= len(args) {
				fatal(fmt.Errorf("--env requires KEY=VALUE"))
			}
			parts := strings.SplitN(args[i], "=", 2)
			if len(parts) != 2 {
				fatal(fmt.Errorf("--env value must be KEY=VALUE"))
			}
			env[parts[0]] = parts[1]
		default:
			fatal(fmt.Errorf("unknown flag: %s", args[i]))
		}
	}

	if image == "" {
		fatal(fmt.Errorf("--image is required"))
	}
	if domain == "" {
		fatal(fmt.Errorf("--domain is required"))
	}
	if healthPath == "" {
		healthPath = "/up"
	}

	// Generate private key
	privateKey, err := matcha.GeneratePrivateKey()
	if err != nil {
		fatal(fmt.Errorf("failed to generate private key: %w", err))
	}
	env["PRIVATE_KEY"] = privateKey

	app := matcha.AppConfig{
		Image:      image,
		Domain:     domain,
		Port:       port,
		HealthPath: healthPath,
		Volumes:    volumes,
		Env:        env,
	}

	if err := matcha.SaveApp(name, app); err != nil {
		fatal(fmt.Errorf("failed to save app config: %w", err))
	}

	// Create data directories for volumes
	for _, v := range matcha.ResolveVolumes(name, volumes) {
		hostPath := strings.SplitN(v, ":", 2)[0]
		if err := os.MkdirAll(hostPath, 0755); err != nil {
			fatal(fmt.Errorf("failed to create volume dir: %w", err))
		}
	}

	fmt.Printf("App %q registered.\n", name)
	fmt.Printf("  Image:   %s\n", image)
	fmt.Printf("  Domain:  %s\n", domain)
	fmt.Printf("  Port:    %d\n", port)
	if len(volumes) > 0 {
		fmt.Printf("  Volumes: %s\n", strings.Join(volumes, ", "))
	}
	fmt.Println()
	fmt.Printf("Deploy with: matcha deploy %s\n", name)
}

func cmdDeploy() {
	name := requireAppName("deploy")
	app, err := matcha.LoadApp(name)
	if err != nil {
		fatal(fmt.Errorf("app %q not found. Run 'matcha list' to see registered apps", name))
	}
	m := matcha.NewFromApp(name, app, matcha.Config{})

	if err := m.Deploy(); err != nil {
		fatal(err)
	}

	fmt.Printf("\nApp %q deployed.\n", name)
	fmt.Printf("\nPoint your domain to this server:\n\n")
	fmt.Printf("    Name:   %s\n", extractSubdomain(app.Domain))
	fmt.Printf("    Type:   A\n")
	fmt.Printf("    Value:  <this server's IP>\n\n")
	fmt.Printf("SSL activates automatically once DNS propagates.\n")
}

func checkCLIUpdate() {
	m := matcha.New(matcha.Config{
		Name:           "matcha",
		BinaryPath:     "/usr/local/bin/matcha",
		ManagerRepo:    "karloscodes/matcha",
		ManagerVersion: version,
	})
	// SelfUpdate re-execs on success, so the restarted process
	// runs `matcha update <name>` again with the new binary.
	_, err := m.SelfUpdate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: CLI self-update check failed: %v\n", err)
	}
}

func cmdUpdate() {
	checkCLIUpdate()

	name := requireAppName("update")
	m := matchaFromConfig(name)

	if err := m.Update(); err != nil {
		fatal(err)
	}
}

func cmdUpdateAll() {
	checkCLIUpdate()

	apps, err := matcha.ListApps()
	if err != nil {
		fatal(err)
	}

	for _, name := range matcha.ListAppsSorted(apps) {
		m := matchaFromConfig(name)
		if err := m.Update(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update %s: %v\n", name, err)
		}
	}
}

func cmdList() {
	apps, err := matcha.ListApps()
	if err != nil {
		fatal(err)
	}

	if len(apps) == 0 {
		fmt.Println("No apps registered. Run 'matcha add' to register one.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tIMAGE\tDOMAIN\tPORT")
	for _, name := range matcha.ListAppsSorted(apps) {
		app := apps[name]
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", name, app.Image, app.Domain, app.Port)
	}
	w.Flush()
}

func cmdStatus() {
	name := requireAppName("status")
	m := matchaFromConfig(name)

	if err := m.Status(); err != nil {
		fatal(err)
	}
}

func cmdRemove() {
	name := requireAppName("remove")
	m := matchaFromConfig(name)

	// Remove from proxy first, then stop container, then clean up config
	if err := m.RemoveFromProxy(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not remove from proxy: %v\n", err)
	}

	if err := m.StopApp(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not stop app: %v\n", err)
	}

	if err := matcha.RemoveApp(name); err != nil {
		fatal(err)
	}

	fmt.Printf("App %q removed.\n", name)
	fmt.Printf("Note: Data directory %s was not removed. Delete manually if no longer needed.\n", matcha.DataDir(name))
}

func cmdExec() {
	name := requireAppName("exec")
	if len(os.Args) < 4 {
		fatal(fmt.Errorf("exec requires a command. Usage: matcha exec <name> <cmd...>"))
	}
	m := matchaFromConfig(name)

	if err := m.Exec(os.Args[3:]...); err != nil {
		fatal(err)
	}
}

func cmdLogs() {
	name := requireAppName("logs")
	m := matchaFromConfig(name)

	if err := m.Logs(); err != nil {
		fatal(err)
	}
}

func cmdCheck() {
	if err := matcha.Check(); err != nil {
		os.Exit(1)
	}
}

