package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const serviceName = "agent-hive"
const serviceFilePath = "/etc/systemd/system/agent-hive.service"

var serviceTemplate = template.Must(template.New("service").Parse(`[Unit]
Description=Agent Hive - Terminal Agent Manager
After=network.target

[Service]
Type=simple
ExecStart={{.ExecStart}}
WorkingDirectory={{.WorkDir}}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`))

type serviceTemplateData struct {
	ExecStart string
	WorkDir   string
}

// CmdInstall installs the systemd service.
func CmdInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	configPath := fs.String("config", "config.yaml", "config file path")
	fs.Parse(args)

	requireRoot("install")

	// Resolve absolute paths
	absConfig, err := filepath.Abs(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(absConfig); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: config file %q does not exist\n", absConfig)
		os.Exit(1)
	}

	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot determine executable path: %v\n", err)
		os.Exit(1)
	}
	exePath, _ = filepath.EvalSymlinks(exePath)

	// Check if service already exists
	if _, err := os.Stat(serviceFilePath); err == nil {
		fmt.Printf("service file already exists at %s. Overwrite? [y/N] ", serviceFilePath)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("aborted")
			return
		}
	}

	// Validate paths have no spaces (systemd ExecStart parsing is fragile with spaces)
	if strings.ContainsAny(exePath, " \t") {
		fmt.Fprintf(os.Stderr, "error: binary path %q contains spaces, which is not supported by systemd ExecStart\n", exePath)
		os.Exit(1)
	}
	if strings.ContainsAny(absConfig, " \t") {
		fmt.Fprintf(os.Stderr, "error: config path %q contains spaces, which is not supported by systemd ExecStart\n", absConfig)
		os.Exit(1)
	}

	// Generate service file
	data := serviceTemplateData{
		ExecStart: exePath + " run --config " + absConfig,
		WorkDir:   filepath.Dir(exePath),
	}

	var buf strings.Builder
	if err := serviceTemplate.Execute(&buf, data); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(serviceFilePath, []byte(buf.String()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing service file: %v\n", err)
		os.Exit(1)
	}

	// daemon-reload and enable
	runSystemctl("daemon-reload")
	runSystemctl("enable", serviceName)

	fmt.Printf("service installed and enabled at %s\n", serviceFilePath)
}

func requireRoot(cmd string) {
	if err := checkRoot(cmd); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// checkRoot returns an error if not running as root. Testable.
func checkRoot(cmd string) error {
	if os.Getuid() != 0 {
		return fmt.Errorf("error: '%s' requires root privileges. Use sudo.", cmd)
	}
	return nil
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RenderServiceFile generates the service file content for testing.
func RenderServiceFile(execStart, workDir string) (string, error) {
	data := serviceTemplateData{
		ExecStart: execStart,
		WorkDir:   workDir,
	}
	var buf strings.Builder
	if err := serviceTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
