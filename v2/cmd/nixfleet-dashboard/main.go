package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/markus-barta/nixfleet/v2/internal/dashboard"
	"github.com/rs/zerolog"
)

func main() {
	// Set up logging
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Timestamp().Logger()

	// Load configuration
	cfg, err := dashboard.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	// Initialize database
	db, err := dashboard.InitDatabase(cfg.DatabasePath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize database")
	}
	defer func() { _ = db.Close() }()

	// Create server
	server := dashboard.New(cfg, db, log)

	// Handle shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info().Msg("shutting down...")
		os.Exit(0)
	}()

	// Run server
	if err := server.Run(); err != nil {
		log.Fatal().Err(err).Msg("server error")
	}
}

