package server

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/m0nsterrr/external-dns-webhook-infomaniak/internal/config"
	"github.com/m0nsterrr/external-dns-webhook-infomaniak/internal/webhook"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"sigs.k8s.io/external-dns/provider/webhook/api"
)

func Init(cfg config.Config, webhookServer api.WebhookServer) (*http.Server, *http.Server) {
	wh := webhook.New(webhookServer.Provider)

	appRouter := chi.NewRouter()
	appRouter.Get("/", wh.Negotiate)
	appRouter.Get("/records", wh.Records)
	appRouter.Post("/records", wh.ApplyChanges)
	appRouter.Post("/adjustendpoints", wh.AdjustEndpoints)
	appServer := createServer(fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort), appRouter, cfg.ServerReadTimeout, cfg.ServerWriteTimeout)

	go func() {
		slog.Info("starting app server", "address", appServer.Addr)
		if err := appServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("could not listen on %s: %v\n", appServer.Addr, err)
		}
	}()

	healthRouter := chi.NewRouter()
	healthRouter.Get("/healthz", healthCheckHandler)
	healthRouter.Get("/readyz", healthCheckHandler)
	healthRouter.Get("/metrics", promhttp.Handler().ServeHTTP)
	healthServer := createServer(fmt.Sprintf("%s:%d", cfg.MetricsHost, cfg.MetricsPort), healthRouter, cfg.ServerReadTimeout, cfg.ServerWriteTimeout)

	go func() {
		slog.Info("starting health server", "address", healthServer.Addr)
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("could not listen on %s: %v\n", healthServer.Addr, err)
		}
	}()

	return appServer, healthServer
}

func createServer(addr string, handler http.Handler, readTimeout, writeTimeout time.Duration) *http.Server {
	return &http.Server{Addr: addr, Handler: handler, ReadTimeout: readTimeout, WriteTimeout: writeTimeout}
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Error("error writing response", "error", err)
	}
}

func ShutdownGracefully(mainServer *http.Server, healthServer *http.Server) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-sigCh

	slog.Error("shutting down servers due to received signal", "error", sig)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := mainServer.Shutdown(ctx); err != nil {
		slog.Error("error shutting down main server", "error", err)
	}

	if err := healthServer.Shutdown(ctx); err != nil {
		slog.Error("error shutting down health server", "error", err)
	}
}
