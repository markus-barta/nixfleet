// NixFleet Agent v2 - Go implementation with WebSocket support.
package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/markus-barta/nixfleet/v2/internal/agent"
	"github.com/markus-barta/nixfleet/v2/internal/config"
	"github.com/rs/zerolog"
)

func main() {
	// Set up logging
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().
		Timestamp().
		Logger()

	// Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	// Set log level
	switch cfg.LogLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	log.Info().
		Str("version", agent.Version).
		Str("hostname", cfg.Hostname).
		Str("url", cfg.DashboardURL).
		Msg("NixFleet Agent starting")

	// Create agent
	a := agent.New(cfg, log)

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Info().Str("signal", sig.String()).Msg("received signal")
		a.Shutdown()
	}()

	// Run agent
	if err := a.Run(); err != nil {
		log.Fatal().Err(err).Msg("agent failed")
	}
}

