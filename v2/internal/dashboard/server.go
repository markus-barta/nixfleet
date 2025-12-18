package dashboard

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
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
	flakeUpdates   *FlakeUpdateService // P5300: Automated flake updates
	router         *chi.Mux
	wsUpgrader     *websocket.Upgrader
	httpServer     *http.Server

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

	s := &Server{
		cfg:            cfg,
		db:             db,
		log:            log.With().Str("component", "dashboard").Logger(),
		auth:           NewAuthService(cfg, db),
		hub:            hub,
		logStore:       logStore,
		versionFetcher: versionFetcher,
		flakeUpdates:   flakeUpdates,
		hubCtx:         hubCtx,
		hubCancel:      hubCancel,
	}

	s.setupRouter()

	// Start hub immediately (auto-recovery enabled, graceful shutdown via hubCtx)
	go s.hub.Run(hubCtx)

	// Start flake update service if configured (P5300)
	if s.flakeUpdates != nil {
		go s.flakeUpdates.Start(hubCtx)
	}

	return s
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
			r.Delete("/hosts/{hostID}", s.handleDeleteHost)
			r.Get("/hosts/{hostID}/logs", s.handleGetLogs)

			// Flake updates (P5300)
			r.Route("/flake-updates", func(r chi.Router) {
				r.Get("/status", s.handleFlakeUpdateStatus)
				r.Post("/check", s.handleFlakeUpdateCheck)
				r.Post("/merge-and-deploy", s.handleMergeAndDeploy)
			})
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
