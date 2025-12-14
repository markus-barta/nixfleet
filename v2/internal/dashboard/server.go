package dashboard

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// Server is the main dashboard server.
type Server struct {
	cfg        *Config
	db         *sql.DB
	log        zerolog.Logger
	auth       *AuthService
	hub        *Hub
	logStore   *LogStore
	router     *chi.Mux
	wsUpgrader *websocket.Upgrader
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

	hub := NewHub(log, db)
	hub.logStore = logStore // Pass log store to hub for output logging

	s := &Server{
		cfg:      cfg,
		db:       db,
		log:      log.With().Str("component", "dashboard").Logger(),
		auth:     NewAuthService(cfg, db),
		hub:      hub,
		logStore: logStore,
	}

	s.setupRouter()

	// Start hub immediately (for testing and normal use)
	go s.hub.Run()

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
			r.Post("/hosts/{hostID}/command", s.handleCommand)
			r.Delete("/hosts/{hostID}", s.handleDeleteHost)
			r.Get("/hosts/{hostID}/logs", s.handleGetLogs)
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

// Run starts the server.
func (s *Server) Run() error {
	s.log.Info().Str("addr", s.cfg.ListenAddr).Msg("starting dashboard server")
	return http.ListenAndServe(s.cfg.ListenAddr, s.router)
}

// Router returns the HTTP router (for testing).
func (s *Server) Router() http.Handler {
	return s.router
}

