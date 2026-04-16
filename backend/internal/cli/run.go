package cli

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/penguin/agent-hive/internal/auth"
	"github.com/penguin/agent-hive/internal/config"
	"github.com/penguin/agent-hive/internal/container"
	"github.com/penguin/agent-hive/internal/logger"
	ptypkg "github.com/penguin/agent-hive/internal/pty"
	"github.com/penguin/agent-hive/internal/server"
	"github.com/penguin/agent-hive/internal/store"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// CmdRun starts the server in foreground mode.
func CmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("config", "config.yaml", "config file path")
	dev := fs.Bool("dev", false, "enable dev mode (proxy to Vite dev server)")
	fs.Parse(args)

	// Setup log file in the binary's directory
	exe, err := os.Executable()
	if err == nil {
		rw, logErr := logger.Setup(filepath.Dir(exe))
		if logErr != nil {
			log.Printf("warning: failed to setup log file: %v", logErr)
		} else {
			defer rw.Close()
		}
	}

	log.Printf("Agent Hive %s", Version)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := store.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	ptyOpts := &ptypkg.SessionOptions{
		User:  cfg.User,
		Shell: cfg.Shell,
	}
	mgr := container.NewManager(cfg.DataDir, ptyOpts)

	metas, err := db.ListContainerMeta()
	if err != nil {
		log.Printf("warning: failed to load containers: %v", err)
	} else {
		for _, m := range metas {
			mgr.Restore(m.ID, m.Name, m.CreatedAt)
			log.Printf("restored container %s (%s) [disconnected]", m.ID, m.Name)
		}
	}

	am := auth.NewManager(cfg.Token, cfg.Machines)
	if am.Enabled() {
		log.Printf("authentication enabled")
	} else {
		log.Printf("authentication disabled (no token in config)")
	}

	srv := server.New(*dev, mgr, db, am)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Local:   http://localhost:%d", cfg.Port)
	for _, ip := range getLocalIPs() {
		log.Printf("Network: http://%s:%d", ip, cfg.Port)
	}

	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}

func getLocalIPs() []string {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			ips = append(ips, ipnet.IP.String())
		}
	}
	return ips
}
