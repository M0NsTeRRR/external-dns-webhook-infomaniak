package main

import (
	"log/slog"
	"os"

	"github.com/m0nsterrr/external-dns-webhook-infomaniak/internal/config"
	"github.com/m0nsterrr/external-dns-webhook-infomaniak/internal/logging"
	"github.com/m0nsterrr/external-dns-webhook-infomaniak/internal/provider"
	"github.com/m0nsterrr/external-dns-webhook-infomaniak/internal/server"
	"sigs.k8s.io/external-dns/provider/webhook/api"
)

var (
	version   = "development"
	buildTime = "0"
)

func main() {
	logging.Init()

	slog.Info("Starting external-dns-webhook-infomaniak", "version", version, "build_time", buildTime)

	config := config.Init()
	provider, err := provider.Init(config)
	if err != nil {
		slog.Error("Failed to initialize DNS provider", "error", err)
		os.Exit(1)
	}

	appServer, healthServer := server.Init(config, api.WebhookServer{Provider: provider})
	server.ShutdownGracefully(appServer, healthServer)
}
