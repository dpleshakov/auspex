package main

import (
	"embed"
	"flag"
	"log"

	"github.com/dpleshakov/auspex/internal/config"
)

// staticFiles holds the compiled frontend, embedded at build time.
// web/dist is produced by `npm run build` inside cmd/auspex/web/.
// Run scripts/build.sh (or build.cmd) to build everything in the correct order.
//
//go:embed all:web/dist
var staticFiles embed.FS

func main() {
	configPath := flag.String("config", "auspex.yaml", "path to config file")
	flag.Parse()

	_, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// TODO(TASK-18): wire up db, store, esi, auth, sync, api.
}
