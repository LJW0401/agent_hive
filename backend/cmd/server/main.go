package main

import (
	"fmt"
	"os"

	"github.com/penguin/agent-hive/internal/cli"
)

var version = "dev"

func main() {
	cli.Version = version

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		cli.CmdRun(os.Args[2:])
	case "init":
		cli.CmdInit(os.Args[2:])
	case "install":
		cli.CmdInstall(os.Args[2:])
	case "uninstall":
		cli.CmdUninstall(os.Args[2:])
	case "start":
		cli.CmdService("start", os.Args[2:])
	case "stop":
		cli.CmdService("stop", os.Args[2:])
	case "restart":
		cli.CmdService("restart", os.Args[2:])
	case "status":
		cli.CmdService("status", os.Args[2:])
	case "logs":
		cli.CmdLogs(os.Args[2:])
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Agent Hive %s

Usage: agent-hive <command> [options]

Commands:
  run          Start the server (foreground)
  init         Generate config.yaml with auto-detected user/shell
  install      Install systemd service
  uninstall    Uninstall systemd service
  start        Start the service
  stop         Stop the service
  restart      Restart the service
  status       Show service status
  logs         View service logs

Run 'agent-hive <command> --help' for more information.
`, cli.Version)
}
