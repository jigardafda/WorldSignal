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
	"time"

	"github.com/worldsignal/backend/internal/config"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/httpapi"
	"github.com/worldsignal/backend/internal/jobs"
	"github.com/worldsignal/backend/internal/llm"
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

	// Auth/RBAC tables + a default admin (first boot only).
	if err := database.MigrateAuth(ctx); err != nil {
		return err
	}
	// Extended source metadata + validation-log tables.
	if err := database.MigrateContent(ctx); err != nil {
		return err
	}
	if created, err := httpapi.SeedDefaultAdmin(ctx, database, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		return err
	} else if created {
		log.Info(fmt.Sprintf("seeded default admin %s (change the password!)", cfg.AdminEmail))
	}

	// The server resolves the effective LLM key (admin-managed DB key, else the
	// env system key); the gateway consults it per request.
	srv := &httpapi.Server{
		DB: database, SigningSecret: cfg.WebhookSigningSecret,
		OpenAIAPIKey: cfg.OpenAIAPIKey, OpenAIModel: cfg.OpenAIModel,
	}
	gateway := llm.NewDynamicGateway(srv.ResolveLLMKey)
	queue := jobs.New(database.Pool)
	workers := jobs.NewWorkers(queue, database, gateway, cfg.WebhookSigningSecret)
	srv.Enqueue = workers

	// Ensure the jobs table exists regardless of role (the API exposes a jobs view).
	if err := queue.Migrate(ctx); err != nil {
		return err
	}

	var scheduler *jobs.Scheduler
	runWorkers := cfg.Role == "all" || cfg.Role == "worker"
	if runWorkers {
		workers.Register()
		queue.Start(ctx)
		scheduler = jobs.NewScheduler(database, workers, time.Duration(cfg.SchedulerTickMS)*time.Millisecond)
		scheduler.Start(ctx)
	}

	var httpSrv *http.Server
	if cfg.Role == "all" || cfg.Role == "api" {
		addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
		httpSrv = &http.Server{Addr: addr, Handler: srv.Handler()}
		go func() {
			log.Info(fmt.Sprintf("API on http://%s  (GraphQL: /graphql, REST: /v1/*)", addr))
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("listen failed", err.Error())
			}
		}()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("shutting down")
	if scheduler != nil {
		scheduler.Stop()
	}
	if runWorkers {
		queue.Stop()
	}
	if httpSrv != nil {
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
