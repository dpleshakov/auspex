package main

import "embed"

// staticFiles holds the compiled frontend, embedded at build time.
// web/dist is produced by `npm run build` inside cmd/auspex/web/.
// Run scripts/build.sh (or build.cmd) to build everything in the correct order.
//
//go:embed all:web/dist
var staticFiles embed.FS

func main() {
}
