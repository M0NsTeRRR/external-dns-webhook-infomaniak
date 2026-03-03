package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/m0nsterrr/external-dns-webhook-infomaniak/internal/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"sigs.k8s.io/external-dns/provider/webhook/api"
)

func Init(config config.Config, webhookServer api.WebhookServer) (*http.Server, *http.Server) {
	appRouter := chi.NewRouter()
	appRouter.HandleFunc("/", webhookServer.NegotiateHandler)
	appRouter.HandleFunc("/records", webhookServer.RecordsHandler)
	appRouter.HandleFunc("/adjustendpoints", webhookServer.AdjustEndpointsHandler)
	appServer := createServer(fmt.Sprintf("%s:%d", config.ServerHost, config.ServerPort), appRouter, config.ServerReadTimeout, config.ServerWriteTimeout)

	go func() {
		log.Printf("starting app server on %s", appServer.Addr)
		if err := appServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("could not listen on %s: %v\n", appServer.Addr, err)
		}
	}()

	healthRouter := chi.NewRouter()
	healthRouter.Get("/healthz", healthCheckHandler)
	healthRouter.Get("/readyz", healthCheckHandler)
	healthRouter.Get("/metrics", promhttp.Handler().ServeHTTP)
	healthServer := createServer(fmt.Sprintf("%s:%d", config.MetricsHost, config.MetricsPort), healthRouter, config.ServerReadTimeout, config.ServerWriteTimeout)

	go func() {
		log.Printf("starting health server on %s", healthServer.Addr)
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
	_, err := w.Write([]byte("OK"))
	if err != nil {
		log.Printf("error writing response: %v", err)
	}
}

func ShutdownGracefully(mainServer *http.Server, healthServer *http.Server) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-sigCh

	log.Printf("shutting down servers due to received signal: %v", sig)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := mainServer.Shutdown(ctx); err != nil {
		log.Printf("error shutting down main server: %v", err)
	}

	if err := healthServer.Shutdown(ctx); err != nil {
		log.Printf("error shutting down health server: %v", err)
	}
}
