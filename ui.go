package matcha

import (
	"fmt"
	"strings"
	"time"
)

// Terminal formatting helpers
func bold(s string) string  { return "\033[1m" + s + "\033[0m" }
func dim(s string) string   { return "\033[2m" + s + "\033[0m" }
func green(s string) string { return "\033[32m" + s + "\033[0m" }
func cyan(s string) string  { return "\033[36m" + s + "\033[0m" }

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner handles animated progress display.
type Spinner struct {
	name   string
	stopCh chan struct{}
	doneCh chan struct{}
}

// StartSpinner creates and starts an animated spinner for a step.
func (m *Matcha) StartSpinner(name string) *Spinner {
	s := &Spinner{
		name:   name,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	// Pad name to 16 chars
	paddedName := name
	for len(paddedName) < 16 {
		paddedName += " "
	}

	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		defer close(s.doneCh)

		idx := 0
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				fmt.Printf("\r\033[K  %s%s", paddedName, dim(spinnerFrames[idx]))
				idx = (idx + 1) % len(spinnerFrames)
			}
		}
	}()

	return s
}

// Stop stops the spinner and shows success or failure.
func (s *Spinner) Stop(success bool) {
	close(s.stopCh)
	<-s.doneCh

	paddedName := s.name
	for len(paddedName) < 16 {
		paddedName += " "
	}

	fmt.Print("\r\033[K")
	if success {
		fmt.Printf("  %s%s\n", paddedName, green("✓"))
	} else {
		fmt.Printf("  %s%s\n", paddedName, "✗")
	}
}

// printWelcome prints the installer welcome message.
func (m *Matcha) printWelcome() {
	fmt.Println()
	// Capitalize first letter for display
	title := m.config.Name
	if len(title) > 0 {
		title = strings.ToUpper(title[:1]) + title[1:]
	}
	fmt.Println(bold(title + " Installer"))
	fmt.Println()
	fmt.Println(dim("* Ports 80 and 443 must be available"))
	fmt.Println(dim("* DNS pointing to this server recommended for SSL"))
	fmt.Println()
}

// printComplete prints the completion message.
func (m *Matcha) printComplete(domain string, dnsWarning bool, serverIP string) {
	fmt.Println()
	fmt.Println(bold("Done."))
	fmt.Println()

	if dnsWarning {
		fmt.Printf("%s Point %s to %s\n", dim("DNS not configured."), domain, serverIP)
		fmt.Println(dim("SSL activates once DNS propagates."))
		fmt.Println()
	}

	fmt.Printf("Visit %s to create your account.\n", cyan("https://"+domain))
}

// printHeader prints a bold header.
func printHeader(text string) {
	fmt.Println()
	fmt.Println(bold(text))
	fmt.Println()
}

// printSuccess prints a success message.
func printSuccess(format string, args ...any) {
	fmt.Printf("%s %s\n", green("✓"), fmt.Sprintf(format, args...))
}

// printWarn prints a warning message.
func printWarn(format string, args ...any) {
	fmt.Printf("  %s %s\n", dim("!"), fmt.Sprintf(format, args...))
}
