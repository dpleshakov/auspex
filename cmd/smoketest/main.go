// cmd/smoketest/main.go
//
// Standalone binary for manual end-to-end verification of the OAuth2 flow
// and ESI connectivity. NOT part of the production application.
//
// Prerequisites:
//   - A valid auspex.yaml in the working directory with:
//       esi:
//         client_id:     <your CCP Developer App client_id>
//         client_secret: <your CCP Developer App client_secret>
//         callback_url:  http://localhost:8081/auth/eve/callback
//       db_path: smoketest.db   # optional; defaults to auspex.db
//
// Usage:
//
//	go run ./cmd/smoketest/
//	Open http://localhost:8081/auth/eve/login in your browser.
//	After successful auth: character name and blueprint count are printed
//	to stdout; the server shuts down automatically.
//
// Lifetime: temporary — deleted once TASK-18 is complete and OAuth is
// verified end-to-end in the full application.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/dpleshakov/auspex/internal/auth"
	"github.com/dpleshakov/auspex/internal/config"
	"github.com/dpleshakov/auspex/internal/db"
	"github.com/dpleshakov/auspex/internal/esi"
	"github.com/dpleshakov/auspex/internal/store"
)

func main() {
	cfg, err := config.Load("auspex.yaml")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer database.Close()

	queries := store.New(database)
	esiClient := esi.NewClient(http.DefaultClient)
	provider := auth.NewProvider(
		cfg.ESI.ClientID,
		cfg.ESI.ClientSecret,
		cfg.ESI.CallbackURL,
		queries,
		http.DefaultClient,
	)

	shutdown := make(chan struct{})

	mux := http.NewServeMux()

	mux.HandleFunc("/auth/eve/login", func(w http.ResponseWriter, r *http.Request) {
		authURL, err := provider.GenerateAuthURL()
		if err != nil {
			http.Error(w, "failed to generate auth URL: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, authURL, http.StatusFound)
	})

	mux.HandleFunc("/auth/eve/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")

		characterID, err := provider.HandleCallback(r.Context(), code, state)
		if err != nil {
			http.Error(w, "callback error: "+err.Error(), http.StatusBadRequest)
			return
		}

		character, err := queries.GetCharacter(r.Context(), characterID)
		if err != nil {
			http.Error(w, "failed to get character from DB: "+err.Error(), http.StatusInternalServerError)
			return
		}

		blueprints, _, err := esiClient.GetCharacterBlueprints(r.Context(), characterID, character.AccessToken)
		if err != nil {
			http.Error(w, "failed to fetch blueprints from ESI: "+err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Printf("\n=== Smoke test passed ===\n")
		fmt.Printf("Character : %s (ID: %d)\n", character.Name, characterID)
		fmt.Printf("Blueprints: %d\n", len(blueprints))
		for i, bp := range blueprints {
			fmt.Printf("  [%d] TypeID=%-8d ME=%2d  TE=%2d  LocationID=%d\n",
				i+1, bp.TypeID, bp.MELevel, bp.TELevel, bp.LocationID)
		}
		fmt.Printf("========================\n\n")

		fmt.Fprintln(w, "Auth successful! Check the terminal for results. Server is shutting down.")

		// Signal shutdown from a goroutine so the response is flushed first.
		go func() { close(shutdown) }()
	})

	srv := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}

	// Shut down when callback completes or OS signal received.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-shutdown:
		case <-sigCh:
		}
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}()

	log.Printf("Smoketest server listening on http://localhost:8081")
	log.Printf("Open http://localhost:8081/auth/eve/login in your browser to start the flow")

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}

	log.Printf("Server shut down cleanly. Smoketest complete.")
}
