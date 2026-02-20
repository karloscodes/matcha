package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/karloscodes/matcha"
)

var version = "0.12.0"

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
	case "migrate":
		cmdMigrate()
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
  list (ls)          List all registered apps
  status <name>      Show app status
  remove (rm) <name> Remove an app
  exec <name> <cmd>  Run a command in the app container
  logs <name>        Stream app logs
  migrate <name>     Migrate from old layout to YAML config
  version            Print version`)
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
		fatal(fmt.Errorf("app %q not found. Run 'matcha list' to see registered apps", name))
	}
	return matcha.New(matcha.Config{
		Name:       name,
		AppImage:   app.Image,
		AppPort:    app.Port,
		HealthPath: app.HealthPath,
		Volumes:    app.Volumes,
	})
}

func cmdSetup() {
	if err := matcha.Setup(); err != nil {
		fatal(err)
	}
	fmt.Println("Matcha is ready. Add your first app with 'matcha add'.")
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
	m := matchaFromConfig(name)

	if err := m.Deploy(); err != nil {
		fatal(err)
	}
	fmt.Printf("App %q deployed.\n", name)
}

func cmdUpdate() {
	name := requireAppName("update")
	m := matchaFromConfig(name)

	if err := m.Update(); err != nil {
		fatal(err)
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

func cmdMigrate() {
	name := requireAppName("migrate")
	m := matcha.New(matcha.Config{
		Name:     name,
		AppImage: "unknown", // will be read from old config
	})
	if err := m.Migrate(); err != nil {
		fatal(err)
	}
	fmt.Printf("Migration complete. Deploy with: matcha deploy %s\n", name)
}
