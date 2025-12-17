package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// Log configuration warnings
	for _, warning := range cfg.Warnings() {
		log.Warn().Msg(warning)
	}

	// Initialize database
	db, err := dashboard.InitDatabase(cfg.DatabasePath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize database")
	}
	defer func() { _ = db.Close() }()

	// Create server
	server := dashboard.New(cfg, db, log)

	// Handle graceful shutdown
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)

	// Run server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Wait for shutdown signal or server error
	select {
	case sig := <-shutdownCh:
		log.Info().Str("signal", sig.String()).Msg("received shutdown signal")
	case err := <-serverErr:
		if err != nil {
			log.Fatal().Err(err).Msg("server error")
		}
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("shutdown error")
		os.Exit(1)
	}

	log.Info().Msg("server shutdown complete")
}
