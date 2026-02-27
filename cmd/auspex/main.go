package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	stdsync "sync"
	"syscall"
	"time"

	"github.com/dpleshakov/auspex/internal/api"
	"github.com/dpleshakov/auspex/internal/auth"
	"github.com/dpleshakov/auspex/internal/config"
	"github.com/dpleshakov/auspex/internal/db"
	"github.com/dpleshakov/auspex/internal/esi"
	"github.com/dpleshakov/auspex/internal/store"
	syncp "github.com/dpleshakov/auspex/internal/sync"
)

// staticFiles holds the compiled frontend, embedded at build time.
// web/dist is produced by `npm run build` inside cmd/auspex/web/.
// Run scripts/build.sh (or build.cmd) to build everything in the correct order.
//
//go:embed all:web/dist
var staticFiles embed.FS

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("db: %v", err)
	}
	defer database.Close() //nolint:errcheck // Close on shutdown, error is inconsequential

	queries := store.New(database)

	esiClient := esi.NewClient(http.DefaultClient)

	authProvider := auth.NewProvider(
		cfg.ESI.ClientID,
		cfg.ESI.ClientSecret,
		cfg.ESI.CallbackURL,
		queries,
		nil,
	)

	// auth.Client wraps esiClient with automatic token injection and refresh.
	// It shares the oauth2.Config from authProvider so credentials are consistent.
	authClient := auth.NewClient(esiClient, queries, authProvider.OAuthConfig(), nil)

	interval := time.Duration(cfg.RefreshInterval) * time.Minute
	worker := syncp.New(queries, authClient, interval)

	distFS, err := fs.Sub(staticFiles, "web/dist")
	if err != nil {
		return fmt.Errorf("preparing static files: %v", err)
	}

	router := api.NewRouter(queries, worker, authProvider, distFS)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start the sync worker in the background.
	// The worker runs an initial cycle immediately, then ticks every RefreshInterval.
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	var wg stdsync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		worker.Run(workerCtx)
	}()

	// Handle SIGINT and SIGTERM for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received signal %s, shutting down", sig)

		// Give in-flight HTTP requests up to 10 seconds to finish.
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}

		// Stop the sync worker and wait for the current cycle to finish.
		cancelWorker()
	}()

	log.Printf("Auspex listening on http://localhost:%d", cfg.Port)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: %v", err)
	}

	// Wait for the sync worker to complete its current cycle before exiting.
	wg.Wait()
	log.Println("shutdown complete")

	return nil
}
