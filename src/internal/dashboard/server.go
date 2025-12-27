package dashboard

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"github.com/markus-barta/nixfleet/internal/colors"
	"github.com/markus-barta/nixfleet/internal/ops"
	"github.com/markus-barta/nixfleet/internal/store"
	"github.com/markus-barta/nixfleet/internal/sync"
	"github.com/rs/zerolog"
)

// Server is the main dashboard server.
type Server struct {
	cfg            *Config
	db             *sql.DB
	log            zerolog.Logger
	auth           *AuthService
	hub            *Hub
	logStore       *LogStore
	versionFetcher *VersionFetcher
	flakeUpdates   *FlakeUpdateService  // P5300: Automated flake updates
	cmdStateMachine *CommandStateMachine // P2800: Command validation state machine (legacy, being replaced)
	nixcfgRepo     *colors.NixcfgRepo   // P2950: Color picker nixcfg integration
	router         *chi.Mux
	wsUpgrader     *websocket.Upgrader
	httpServer     *http.Server

	// v3 Op Engine components (CORE-001 through CORE-004)
	stateStore       *store.StateStore       // CORE-003: Unified persistence
	opRegistry       *ops.Registry           // CORE-001: Op definitions
	opExecutor       *ops.Executor           // CORE-001: Op execution
	pipelineRegistry *ops.PipelineRegistry   // CORE-002: Pipeline definitions
	pipelineExecutor *ops.PipelineExecutor   // CORE-002: Pipeline execution
	stateManager     *sync.StateManager      // CORE-004: State sync protocol

	// Context for hub lifecycle (created in New, canceled in Shutdown)
	hubCtx    context.Context
	hubCancel context.CancelFunc
}

// New creates a new dashboard server.
func New(cfg *Config, db *sql.DB, log zerolog.Logger) *Server {
	// Mark all hosts offline on startup - they'll go online when agents reconnect
	// This prevents stale "online" status from previous dashboard instances
	result, err := db.Exec(`UPDATE hosts SET status = 'offline' WHERE status = 'online'`)
	if err != nil {
		log.Warn().Err(err).Msg("failed to reset host status on startup")
	} else if rows, _ := result.RowsAffected(); rows > 0 {
		log.Info().Int64("count", rows).Msg("marked hosts offline on startup (will reconnect)")
	}

	// Initialize log store
	logsPath := cfg.DataDir + "/logs"
	logStore, err := NewLogStore(logsPath)
	if err != nil {
		log.Warn().Err(err).Msg("failed to initialize log store, logs will not be persisted")
	}

	// Create hub context - used for graceful shutdown
	hubCtx, hubCancel := context.WithCancel(context.Background())

	// Create version fetcher if configured (P5000)
	// Must be created before Hub so it can be passed for Git status in heartbeat broadcasts
	var versionFetcher *VersionFetcher
	if cfg.HasVersionTracking() {
		versionFetcher = NewVersionFetcher(cfg.VersionURL, cfg.VersionFetchTTL)
		versionFetcher.Start(hubCtx)
		log.Info().Str("url", cfg.VersionURL).Msg("version tracking enabled")
	}

	hub := NewHub(log, db, cfg, versionFetcher)
	hub.logStore = logStore // Pass log store to hub for output logging

	// Create flake update service if GitHub is configured (P5300)
	var flakeUpdates *FlakeUpdateService
	if cfg.HasGitHubIntegration() {
		flakeUpdates = NewFlakeUpdateService(cfg, hub, log)
		hub.SetFlakeUpdates(flakeUpdates) // Enable PR status on browser connect
		log.Info().Str("repo", cfg.GitHubRepo).Msg("GitHub flake updates enabled")
	}

	// Create command state machine (P2800) - legacy, being replaced by Op Engine
	cmdStateMachine := NewCommandStateMachine(log, hub)
	hub.SetCommandStateMachine(cmdStateMachine) // P2800: Give hub direct access to state machine

	// v3 Op Engine initialization (CORE-001 through CORE-004)
	// Create state store (wraps existing db with v3 schema)
	stateStore := store.New(log, db)
	if err := stateStore.LoadVersion(); err != nil {
		log.Warn().Err(err).Msg("failed to load state version, starting from 0")
	}

	// Create op and pipeline registries with default ops
	opRegistry := ops.DefaultRegistry()
	pipelineRegistry := ops.DefaultPipelineRegistry()

	// Create command sender adapter for op executor
	cmdSender := &hubCommandSender{hub: hub}

	// Create op executor
	opExecutor := ops.NewExecutor(log, opRegistry, cmdSender, stateStore, stateStore)

	// Create pipeline executor
	pipelineExecutor := ops.NewPipelineExecutor(log, opExecutor, pipelineRegistry, stateStore, stateStore)

	// Create state provider for sync protocol
	stateProvider := NewDashboardStateProvider(db)

	// Create state manager for sync protocol
	stateManager := sync.NewStateManager(log, stateStore, stateProvider)

	log.Info().Msg("v3 Op Engine initialized")

	// Create nixcfg repo manager if color picker integration is configured (P2950)
	var nixcfgRepo *colors.NixcfgRepo
	if cfg.HasColorPickerIntegration() {
		nixcfgRepo = colors.NewNixcfgRepo(
			cfg.NixcfgRepoPath,
			cfg.GitHubRepo,
			cfg.GitHubToken,
			cfg.ColorCommitMode,
			log,
		)
		log.Info().
			Str("path", cfg.NixcfgRepoPath).
			Str("mode", cfg.ColorCommitMode).
			Msg("color picker nixcfg integration enabled")
	}

	s := &Server{
		cfg:              cfg,
		db:               db,
		log:              log.With().Str("component", "dashboard").Logger(),
		auth:             NewAuthService(cfg, db),
		hub:              hub,
		logStore:         logStore,
		versionFetcher:   versionFetcher,
		flakeUpdates:     flakeUpdates,
		cmdStateMachine:  cmdStateMachine,
		nixcfgRepo:       nixcfgRepo,
		stateStore:       stateStore,
		opRegistry:       opRegistry,
		opExecutor:       opExecutor,
		pipelineRegistry: pipelineRegistry,
		pipelineExecutor: pipelineExecutor,
		stateManager:     stateManager,
		hubCtx:           hubCtx,
		hubCancel:        hubCancel,
	}

	s.setupRouter()

	// Start hub immediately (auto-recovery enabled, graceful shutdown via hubCtx)
	go s.hub.Run(hubCtx)

	// Start flake update service if configured (P5300)
	if s.flakeUpdates != nil {
		go s.flakeUpdates.Start(hubCtx)
	}

	// P2800: Start timeout checking loop
	go s.timeoutCheckLoop(hubCtx)

	// v3: Start state sync beacon (CORE-004)
	s.stateManager.StartBeacon()

	// v3: Start cleanup loop for old commands/events (CORE-003)
	go s.cleanupLoop(hubCtx)

	return s
}

// timeoutCheckLoop periodically checks for command timeouts.
func (s *Server) timeoutCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.cmdStateMachine != nil {
				s.cmdStateMachine.CheckTimeouts()
			}
		}
	}
}

// cleanupLoop periodically removes old commands, pipelines, and events.
// Runs once per hour with 7-day retention (CORE-003).
func (s *Server) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	retention := 7 * 24 * time.Hour // 7 days

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.stateStore != nil {
				cmds, _ := s.stateStore.CleanupOldCommands(retention)
				pls, _ := s.stateStore.CleanupOldPipelines(retention)
				evts, _ := s.stateStore.CleanupOldEvents(retention)
				if cmds+pls+evts > 0 {
					s.log.Info().
						Int64("commands", cmds).
						Int64("pipelines", pls).
						Int64("events", evts).
						Msg("cleaned up old records")
				}
			}
		}
	}
}

func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(s.securityHeaders)

	// Static files
	fs := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fs))

	// Public routes
	r.Get("/health", s.handleHealth)
	r.Get("/login", s.handleLoginPage)
	r.Post("/login", s.handleLogin)

	// WebSocket (handles both agents and browsers)
	r.Get("/ws", s.handleWebSocket)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)

		r.Get("/", s.handleDashboard)

		// Logout requires CSRF
		r.With(s.requireCSRF).Post("/logout", s.handleLogout)

		// API routes
		r.Route("/api", func(r chi.Router) {
			r.Use(s.requireCSRF)

			r.Get("/hosts", s.handleGetHosts)
			r.Post("/hosts", s.handleAddHost)
			r.Post("/hosts/{hostID}/command", s.handleCommand)
			r.Post("/hosts/{hostID}/refresh", s.handleRefreshHost) // P7000: Per-host status refresh
			r.Post("/hosts/{hostID}/refresh-git", s.handleRefreshGit) // P2800: Force-refresh git status
			r.Post("/hosts/{hostID}/theme-color", s.handleSetThemeColor) // P2950: Color picker
			r.Post("/hosts/{hostID}/reboot", s.handleReboot)             // P6900: Reboot with TOTP
			r.Delete("/hosts/{hostID}", s.handleDeleteHost)
			r.Get("/hosts/{hostID}/logs", s.handleGetLogs)

			// P2800: Command state machine endpoints
			r.Post("/hosts/{hostID}/kill", s.handleKillCommand)               // Kill running command
			r.Post("/hosts/{hostID}/timeout-action", s.handleTimeoutAction)   // Handle timeout user action
			r.Get("/command-states", s.handleGetCommandStates)                 // Get all command states

			// System log (P2800)
			r.Get("/system-log", s.handleGetSystemLogs)

			// Flake updates (P5300)
			r.Route("/flake-updates", func(r chi.Router) {
				r.Get("/status", s.handleFlakeUpdateStatus)
				r.Post("/check", s.handleFlakeUpdateCheck)
				r.Post("/merge-and-deploy", s.handleMergeAndDeploy)
			})

			// v3 Op Engine API (CORE-001 through CORE-004)
			r.Post("/dispatch", s.handleDispatchOp)            // Execute op(s) on hosts
			r.Post("/dispatch/pipeline", s.handleDispatchPipeline) // Execute pipeline on hosts
			r.Get("/ops", s.handleGetOps)                       // List available ops
			r.Get("/pipelines", s.handleGetPipelines)           // List available pipelines
			r.Get("/events", s.handleGetEventLog)               // Get recent events
			r.Get("/hosts/{hostID}/events", s.handleGetHostEvents) // Get host events
		})
	})

	s.router = r
}

// securityHeaders adds security headers to responses.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// requireAuth middleware checks for valid session.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := s.auth.GetSessionFromRequest(r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Store session in context for handlers
		ctx := withSession(r.Context(), session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireCSRF middleware validates CSRF token for state-changing requests.
func (s *Server) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		session := sessionFromContext(r.Context())
		if session == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token := r.Header.Get("X-CSRF-Token")
		if token == "" {
			token = r.FormValue("csrf_token")
		}

		if !s.auth.ValidateCSRF(session, token) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Run starts the server with graceful shutdown support.
// Note: The hub is already running (started in New()).
func (s *Server) Run() error {
	s.httpServer = &http.Server{
		Addr:    s.cfg.ListenAddr,
		Handler: s.router,
	}

	s.log.Info().Str("addr", s.cfg.ListenAddr).Msg("starting dashboard server")
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server and hub.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info().Msg("shutting down server...")

	// Stop v3 state sync beacon
	if s.stateManager != nil {
		s.stateManager.StopBeacon()
	}

	// Cancel hub context first (stops hub goroutines)
	if s.hubCancel != nil {
		s.hubCancel()
	}

	// Shutdown HTTP server
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// Router returns the HTTP router (for testing).
func (s *Server) Router() http.Handler {
	return s.router
}
