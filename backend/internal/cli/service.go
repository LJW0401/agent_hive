package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

// CmdService handles start/stop/restart/status commands.
func CmdService(action string, args []string) {
	switch action {
	case "start", "stop", "restart":
		requireRoot(action)
		cmd := exec.Command("systemctl", action, serviceName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error: systemctl %s failed: %v\n", action, err)
			os.Exit(1)
		}
		fmt.Printf("service %s: ok\n", action)

	case "status":
		cmd := exec.Command("systemctl", "status", serviceName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run() // status exits non-zero when stopped, which is OK
	}
}

// CmdLogs handles the logs command.
func CmdLogs(args []string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	follow := fs.Bool("f", false, "follow log output")
	lines := fs.String("n", "", "number of lines to show")
	fs.Parse(args)

	journalArgs := []string{"-u", serviceName}
	if *follow {
		journalArgs = append(journalArgs, "-f")
	}
	if *lines != "" {
		journalArgs = append(journalArgs, "-n", *lines)
	}

	cmd := exec.Command("journalctl", journalArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: journalctl failed: %v\n", err)
		os.Exit(1)
	}
}
