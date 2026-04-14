package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/penguin/agent-hive/internal/container"
	"github.com/penguin/agent-hive/internal/server"
	"github.com/penguin/agent-hive/internal/store"
)

func main() {
	port := flag.Int("port", 8090, "server port")
	dev := flag.Bool("dev", false, "enable dev mode (proxy to Vite dev server)")
	dataDir := flag.String("data", "./data", "data directory")
	flag.Parse()

	db, err := store.New(*dataDir)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	mgr := container.NewManager(*dataDir)

	// Restore containers from database (all start as disconnected)
	metas, err := db.ListContainerMeta()
	if err != nil {
		log.Printf("warning: failed to load containers: %v", err)
	} else {
		for _, m := range metas {
			mgr.Restore(m.ID, m.Name, m.CreatedAt)
			log.Printf("restored container %s (%s) [disconnected]", m.ID, m.Name)
		}
	}

	srv := server.New(*dev, mgr, db)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Agent Hive listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}
