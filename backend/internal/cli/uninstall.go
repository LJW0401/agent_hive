package cli

import (
	"fmt"
	"os"
)

// CmdUninstall removes the systemd service.
func CmdUninstall(args []string) {
	requireRoot("uninstall")

	if _, err := os.Stat(serviceFilePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: service is not installed (%s not found)\n", serviceFilePath)
		os.Exit(1)
	}

	// Stop (ignore error — might already be stopped)
	runSystemctl("stop", serviceName)

	// Disable
	runSystemctl("disable", serviceName)

	// Remove service file
	if err := os.Remove(serviceFilePath); err != nil {
		fmt.Fprintf(os.Stderr, "error removing service file: %v\n", err)
		os.Exit(1)
	}

	// Reload daemon
	runSystemctl("daemon-reload")

	fmt.Println("service uninstalled")
}
