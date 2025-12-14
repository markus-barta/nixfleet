package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/markus-barta/nixfleet/v2/internal/templates"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: validate origin in production
	},
}

// handleLoginPage renders the login page.
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	// Check if already logged in
	if _, err := s.auth.GetSessionFromRequest(r); err == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Get error from query param (set after failed login)
	errorMsg := r.URL.Query().Get("error")

	w.Header().Set("Content-Type", "text/html")
	_ = templates.Login(errorMsg).Render(context.Background(), w)
}

// handleLogin processes login form submission.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Check rate limit - normalize IP by stripping port
	ip := r.RemoteAddr
	if colonIdx := strings.LastIndex(ip, ":"); colonIdx != -1 {
		// Check if this is IPv6 in brackets
		if bracketIdx := strings.LastIndex(ip, "]"); bracketIdx == -1 || colonIdx > bracketIdx {
			ip = ip[:colonIdx]
		}
	}
	if s.auth.IsRateLimited(ip) {
		http.Redirect(w, r, "/login?error=Too+many+attempts.+Please+wait.", http.StatusFound)
		return
	}

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?error=Invalid+request", http.StatusFound)
		return
	}

	password := r.FormValue("password")
	totpCode := r.FormValue("totp")

	// Check password
	if !s.auth.CheckPassword(password) {
		s.log.Warn().Str("ip", ip).Msg("failed login attempt: wrong password")
		http.Redirect(w, r, "/login?error=Invalid+password", http.StatusFound)
		return
	}

	// Check TOTP if configured
	if s.cfg.HasTOTP() && !s.auth.CheckTOTP(totpCode) {
		s.log.Warn().Str("ip", ip).Msg("failed login attempt: wrong TOTP")
		http.Redirect(w, r, "/login?error=Invalid+TOTP+code", http.StatusFound)
		return
	}

	// Create session
	session, err := s.auth.CreateSession()
	if err != nil {
		s.log.Error().Err(err).Msg("failed to create session")
		http.Redirect(w, r, "/login?error=Server+error", http.StatusFound)
		return
	}

	// Reset rate limit on success
	s.auth.ResetRateLimit(ip)

	// Set cookie and redirect
	s.auth.SetSessionCookie(w, session)
	http.Redirect(w, r, "/", http.StatusFound)
}

// handleLogout logs the user out.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	session := sessionFromContext(r.Context())
	if session != nil {
		_ = s.auth.DeleteSession(session.ID)
	}
	s.auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// handleDashboard renders the main dashboard.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	session := sessionFromContext(r.Context())

	// Fetch hosts from database
	hosts, err := s.getHosts()
	if err != nil {
		s.log.Error().Err(err).Msg("failed to fetch hosts")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Count online hosts
	onlineCount := 0
	for _, h := range hosts {
		if h.Online {
			onlineCount++
		}
	}

	data := templates.DashboardData{
		Hosts: hosts,
		Stats: templates.Stats{
			Online: onlineCount,
			Total:  len(hosts),
		},
		CSRFToken: session.CSRFToken,
		Version:   "2.0.0", // TODO: inject from build
	}

	w.Header().Set("Content-Type", "text/html")
	_ = templates.Dashboard(data).Render(context.Background(), w)
}

// getHosts fetches all hosts from the database
func (s *Server) getHosts() ([]templates.Host, error) {
	rows, err := s.db.Query(`
		SELECT id, hostname, host_type, agent_version, os_version, 
		       nixpkgs_version, generation, last_seen, status, pending_command, theme_color
		FROM hosts ORDER BY hostname
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var hosts []templates.Host
	for rows.Next() {
		var h struct {
			ID, Hostname, HostType                                  string
			AgentVersion, OSVersion, NixpkgsVersion, Generation     *string
			LastSeen                                                *string
			Status                                                  string
			PendingCommand, ThemeColor                              *string
		}
		if err := rows.Scan(&h.ID, &h.Hostname, &h.HostType, &h.AgentVersion,
			&h.OSVersion, &h.NixpkgsVersion, &h.Generation, &h.LastSeen,
			&h.Status, &h.PendingCommand, &h.ThemeColor); err != nil {
			continue
		}

		host := templates.Host{
			ID:       h.ID,
			Hostname: h.Hostname,
			HostType: h.HostType,
			Status:   h.Status,
			Online:   h.Status == "online" || h.Status == "running",
		}
		if h.AgentVersion != nil {
			host.AgentVersion = *h.AgentVersion
		}
		if h.OSVersion != nil {
			host.OSVersion = *h.OSVersion
		}
		if h.Generation != nil {
			host.Generation = *h.Generation
		}
		if h.LastSeen != nil {
			host.LastSeen = *h.LastSeen
		}
		if h.PendingCommand != nil {
			host.PendingCommand = *h.PendingCommand
		}
		if h.ThemeColor != nil {
			host.ThemeColor = *h.ThemeColor
		}
		hosts = append(hosts, host)
	}
	return hosts, nil
}

// handleWebSocket handles both agent and browser WebSocket connections.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check authentication: Bearer token for agents, session cookie for browsers
	var clientType string
	var clientID string

	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if !s.auth.ValidateAgentToken(token) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		clientType = "agent"
		clientID = "" // Will be set after registration
	} else {
		session, err := s.auth.GetSessionFromRequest(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		clientType = "browser"
		clientID = session.ID
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}

	// Register with hub
	client := &Client{
		conn:       conn,
		clientType: clientType,
		clientID:   clientID,
		send:       make(chan []byte, 256),
		hub:        s.hub,
		server:     s,
	}

	s.hub.register <- client
	go client.writePump()
	go client.readPump()
}

// handleGetHosts returns the list of hosts.
func (s *Server) handleGetHosts(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT id, hostname, host_type, agent_version, os_version, 
		       nixpkgs_version, generation, last_seen, status, pending_command, comment
		FROM hosts ORDER BY hostname
	`)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to query hosts")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	var hosts []map[string]any
	for rows.Next() {
		var h struct {
			ID, Hostname, HostType                                                 string
			AgentVersion, OSVersion, NixpkgsVersion, Generation                    *string
			LastSeen                                                               *string
			Status                                                                 string
			PendingCommand, Comment                                                *string
		}
		if err := rows.Scan(&h.ID, &h.Hostname, &h.HostType, &h.AgentVersion,
			&h.OSVersion, &h.NixpkgsVersion, &h.Generation, &h.LastSeen,
			&h.Status, &h.PendingCommand, &h.Comment); err != nil {
			continue
		}
		hosts = append(hosts, map[string]any{
			"id":              h.ID,
			"hostname":        h.Hostname,
			"host_type":       h.HostType,
			"agent_version":   h.AgentVersion,
			"os_version":      h.OSVersion,
			"nixpkgs_version": h.NixpkgsVersion,
			"generation":      h.Generation,
			"last_seen":       h.LastSeen,
			"status":          h.Status,
			"pending_command": h.PendingCommand,
			"comment":         h.Comment,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"hosts": hosts})
}

// handleCommand dispatches a command to an agent.
func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "hostID")

	var req struct {
		Command string   `json:"command"`
		Args    []string `json:"args,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Find agent connection for this host
	agent := s.hub.GetAgent(hostID)
	if agent == nil {
		http.Error(w, "Host offline", http.StatusConflict)
		return
	}

	// Send command to agent
	cmdMsg, err := json.Marshal(map[string]any{
		"type": "command",
		"payload": map[string]any{
			"command": req.Command,
			"args":    req.Args,
		},
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	select {
	case agent.send <- cmdMsg:
		// Command sent
	default:
		http.Error(w, "Agent not responsive", http.StatusServiceUnavailable)
		return
	}

	// Update host status
	_, _ = s.db.Exec(`UPDATE hosts SET pending_command = ?, status = 'running' WHERE id = ?`,
		req.Command, hostID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "queued",
		"command": req.Command,
	})

	// Broadcast to browsers
	s.hub.BroadcastToBrowsers(map[string]any{
		"type": "command_queued",
		"payload": map[string]any{
			"host_id": hostID,
			"command": req.Command,
		},
	})
}

// handleGetLogs returns command logs for a host.
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "hostID")

	rows, err := s.db.Query(`
		SELECT id, command, status, exit_code, message, started_at, completed_at
		FROM command_logs WHERE host_id = ? ORDER BY started_at DESC LIMIT 50
	`, hostID)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to query logs")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	var logs []map[string]any
	for rows.Next() {
		var l struct {
			ID                    int
			Command, Status       string
			ExitCode              *int
			Message               *string
			StartedAt, CompletedAt string
		}
		if err := rows.Scan(&l.ID, &l.Command, &l.Status, &l.ExitCode,
			&l.Message, &l.StartedAt, &l.CompletedAt); err != nil {
			continue
		}
		logs = append(logs, map[string]any{
			"id":           l.ID,
			"command":      l.Command,
			"status":       l.Status,
			"exit_code":    l.ExitCode,
			"message":      l.Message,
			"started_at":   l.StartedAt,
			"completed_at": l.CompletedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"logs": logs})
}

