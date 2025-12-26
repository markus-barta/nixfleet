// NixFleet Agent v2 - Go implementation with WebSocket support.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/markus-barta/nixfleet/v2/internal/agent"
	"github.com/markus-barta/nixfleet/v2/internal/config"
	"github.com/rs/zerolog"
)

func main() {
	// CLI flags
	showVersion := flag.Bool("version", false, "print version and exit")
	showHelp := flag.Bool("help", false, "show usage")
	runCheck := flag.Bool("check", false, "validate config and test connectivity")

	// Short flags
	flag.BoolVar(showVersion, "v", false, "print version and exit")
	flag.BoolVar(showHelp, "h", false, "show usage")

	flag.Usage = printUsage
	flag.Parse()

	if *showVersion {
		fmt.Printf("nixfleet-agent %s\n", agent.Version)
		os.Exit(0)
	}

	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	if *runCheck {
		os.Exit(runConfigCheck())
	}

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

func printUsage() {
	fmt.Printf(`Usage: nixfleet-agent [options]

NixFleet Agent %s - connects to NixFleet dashboard for fleet management.

Options:
  -v, --version   Print version and exit
  -h, --help      Print this help and exit
  --check         Validate config and test connectivity

Environment variables:
  NIXFLEET_URL              Dashboard WebSocket URL (required)
  NIXFLEET_TOKEN            Authentication token (required)
  NIXFLEET_REPO_URL         Git repository URL (for isolated mode)
  NIXFLEET_REPO_DIR         Local repository path
  NIXFLEET_BRANCH           Git branch (default: main)
  NIXFLEET_SSH_KEY          SSH key path for git operations
  NIXFLEET_HOSTNAME         Override hostname detection
  NIXFLEET_INTERVAL         Heartbeat interval in seconds (default: 5)
  NIXFLEET_LOG_LEVEL        Log level: debug, info, warn, error
  NIXFLEET_THEME_COLOR      Host theme color (hex, e.g. #7aa2f7)
  NIXFLEET_LOCATION         Location: home, work, cloud
  NIXFLEET_DEVICE_TYPE      Device type: server, desktop, laptop, gaming
`, agent.Version)
}

func runConfigCheck() int {
	fmt.Println("Checking configuration...")
	fmt.Println()

	// Load config
	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Printf("❌ Config error: %v\n", err)
		return 1
	}

	fmt.Println("✓ Config OK")
	fmt.Printf("  Hostname:    %s\n", cfg.Hostname)
	fmt.Printf("  Dashboard:   %s\n", cfg.DashboardURL)
	fmt.Printf("  Repo Dir:    %s\n", cfg.RepoDir)
	if cfg.RepoURL != "" {
		fmt.Printf("  Repo URL:    %s\n", cfg.RepoURL)
	}
	fmt.Printf("  Branch:      %s\n", cfg.Branch)
	fmt.Println()

	// Test connectivity
	fmt.Print("Testing dashboard connectivity... ")

	// Convert WebSocket URL to HTTP for health check
	httpURL := cfg.DashboardURL
	httpURL = strings.Replace(httpURL, "wss://", "https://", 1)
	httpURL = strings.Replace(httpURL, "ws://", "http://", 1)
	// Strip /ws/agent suffix if present
	httpURL = strings.TrimSuffix(httpURL, "/ws/agent")
	httpURL = strings.TrimSuffix(httpURL, "/ws")

	client := &http.Client{Timeout: 10 * time.Second}
	start := time.Now()
	resp, err := client.Get(httpURL)
	latency := time.Since(start)

	if err != nil {
		fmt.Printf("❌ Failed\n")
		fmt.Printf("  Error: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		fmt.Printf("❌ Failed (HTTP %d)\n", resp.StatusCode)
		return 1
	}

	fmt.Printf("✓ OK (latency: %dms)\n", latency.Milliseconds())
	return 0
}
