// Command server is the WorldSignal Go backend entrypoint.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/worldsignal/backend/internal/config"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/httpapi"
	"github.com/worldsignal/backend/internal/logging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	log := logging.New("server")
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()
	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()

	log.Info(fmt.Sprintf("starting WorldSignal (role=%s, llm=%s)", cfg.Role, llmMode(cfg)))

	srv := &httpapi.Server{
		DB:            database,
		Enqueue:       noopEnqueuer{}, // workers wired in Phase 4
		SigningSecret: cfg.WebhookSigningSecret,
	}

	if cfg.Role == "all" || cfg.Role == "api" {
		addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
		httpSrv := &http.Server{Addr: addr, Handler: srv.Handler()}

		go func() {
			log.Info(fmt.Sprintf("API on http://%s  (GraphQL: /graphql, REST: /v1/*)", addr))
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("listen failed", err.Error())
			}
		}()

		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		log.Info("shutting down")
		_ = httpSrv.Shutdown(ctx)
	}
	return nil
}

func llmMode(c config.Config) string {
	if c.HasOpenAI() {
		return "openai"
	}
	return "heuristic-fallback"
}

type noopEnqueuer struct{}

func (noopEnqueuer) EnqueueFetchSource(string) error { return nil }
