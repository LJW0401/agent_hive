package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/penguin/agent-hive/internal/auth"
	"github.com/penguin/agent-hive/internal/config"
	"github.com/penguin/agent-hive/internal/container"
	"github.com/penguin/agent-hive/internal/server"
	"github.com/penguin/agent-hive/internal/store"
)

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	dev := flag.Bool("dev", false, "enable dev mode (proxy to Vite dev server)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := store.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	mgr := container.NewManager(cfg.DataDir)

	// Restore containers from database
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
	log.Printf("Agent Hive listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}
