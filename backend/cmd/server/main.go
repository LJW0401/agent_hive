package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/penguin/agent-hive/internal/server"
)

func main() {
	port := flag.Int("port", 8090, "server port")
	dev := flag.Bool("dev", false, "enable dev mode (proxy to Vite dev server)")
	flag.Parse()

	srv := server.New(*dev)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Agent Hive listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}
