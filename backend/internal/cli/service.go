package cli

import (
	"fmt"
)

// CmdService handles start/stop/restart/status commands.
func CmdService(action string, args []string) {
	fmt.Printf("%s: not yet implemented\n", action)
}

// CmdLogs handles the logs command.
func CmdLogs(args []string) {
	fmt.Println("logs: not yet implemented")
}
