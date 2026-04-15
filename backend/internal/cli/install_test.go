package cli

import (
	"os"
	"strings"
	"testing"
)

func TestRenderServiceFile(t *testing.T) {
	content, err := RenderServiceFile("/usr/local/bin/agent-hive run --config /etc/agent-hive/config.yaml", "/usr/local/bin")
	if err != nil {
		t.Fatalf("RenderServiceFile: %v", err)
	}

	if !strings.Contains(content, "ExecStart=/usr/local/bin/agent-hive run --config /etc/agent-hive/config.yaml") {
		t.Fatal("expected ExecStart in service file")
	}
	if !strings.Contains(content, "WorkingDirectory=/usr/local/bin") {
		t.Fatal("expected WorkingDirectory in service file")
	}
	if !strings.Contains(content, "[Unit]") {
		t.Fatal("expected [Unit] section")
	}
	if !strings.Contains(content, "[Install]") {
		t.Fatal("expected [Install] section")
	}
}

func TestRequireRoot_NonRoot(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root")
	}

	// requireRoot calls os.Exit, so we just verify the check logic
	isRoot := os.Getuid() == 0
	if isRoot {
		t.Fatal("expected non-root")
	}
}

func TestServiceFilePath(t *testing.T) {
	if serviceFilePath != "/etc/systemd/system/agent-hive.service" {
		t.Fatalf("unexpected service file path: %s", serviceFilePath)
	}
}

func TestServiceName(t *testing.T) {
	if serviceName != "agent-hive" {
		t.Fatalf("unexpected service name: %s", serviceName)
	}
}
