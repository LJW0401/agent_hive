package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/penguin/agent-hive/internal/config"
	"gopkg.in/yaml.v3"
)

// CmdInit generates a config.yaml file with auto-detected user/shell.
func CmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	userName := fs.String("user", "", "username (default: current user)")
	shellPath := fs.String("shell", "", "shell path (default: user's default shell)")
	fs.Parse(args)

	// Determine user
	targetUser := *userName
	if targetUser == "" {
		u, err := user.Current()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot detect current user: %v\n", err)
			os.Exit(1)
		}
		targetUser = u.Username
	} else {
		if _, err := user.Lookup(targetUser); err != nil {
			fmt.Fprintf(os.Stderr, "error: user %q does not exist\n", targetUser)
			os.Exit(1)
		}
	}

	// Determine shell
	targetShell := *shellPath
	if targetShell == "" {
		targetShell = config.LookupUserShell(targetUser)
	} else {
		if _, err := os.Stat(targetShell); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error: shell %q does not exist\n", targetShell)
			os.Exit(1)
		}
	}

	// Check existing file
	const configFile = "config.yaml"
	if _, err := os.Stat(configFile); err == nil {
		fmt.Printf("config.yaml already exists. Overwrite? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("aborted")
			return
		}
	}

	cfg := config.DefaultConfig()
	cfg.User = targetUser
	cfg.Shell = targetShell

	data, err := yaml.Marshal(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("config.yaml created (user=%s, shell=%s)\n", targetUser, targetShell)
}
