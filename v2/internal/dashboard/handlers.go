package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/markus-barta/nixfleet/v2/internal/templates"
)

// upgrader returns the WebSocket upgrader (lazily initialized).
func (s *Server) upgrader() *websocket.Upgrader {
	if s.wsUpgrader == nil {
		s.wsUpgrader = &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     s.checkOrigin,
		}
	}
	return s.wsUpgrader
}

// checkOrigin validates the Origin header for WebSocket connections.
// Returns true if the origin is allowed, false otherwise.
func (s *Server) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")

	// No origin header = same-origin request (non-browser or same-origin browser)
	if origin == "" {
		return true
	}

	// Parse origin URL
	originURL, err := url.Parse(origin)
	if err != nil {
		s.log.Warn().Str("origin", origin).Msg("rejected WebSocket: invalid origin URL")
		return false
	}

	// Check against explicitly allowed origins first
	for _, allowed := range s.cfg.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}

	// Get request host (what the browser thinks it's connecting to)
	host := r.Host

	// Development: allow localhost variants
	if isLocalhost(host) {
		if isLocalhost(originURL.Host) {
			return true
		}
		s.log.Warn().
			Str("origin", origin).
			Str("host", host).
			Msg("rejected WebSocket: localhost host but non-localhost origin")
		return false
	}

	// Production: origin must match request host with HTTPS
	expectedOrigin := fmt.Sprintf("https://%s", host)
	if origin == expectedOrigin {
		return true
	}

	s.log.Warn().
		Str("origin", origin).
		Str("expected", expectedOrigin).
		Msg("rejected WebSocket: origin mismatch")
	return false
}

// isLocalhost checks if the host is a localhost variant.
func isLocalhost(host string) bool {
	// Strip port if present
	if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
		// Handle IPv6 addresses in brackets
		if bracketIdx := strings.LastIndex(host, "]"); bracketIdx == -1 || colonIdx > bracketIdx {
			host = host[:colonIdx]
		}
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// handleHealth returns a simple health check response.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
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
	s.auth.SetSessionCookie(w, r, session)
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
		CSRFToken:         session.CSRFToken,
		Version:           VersionInfo(),
		HeartbeatInterval: 5, // Matches host configs (5s heartbeat)
	}

	w.Header().Set("Content-Type", "text/html")
	_ = templates.Dashboard(data).Render(context.Background(), w)
}

// getHosts fetches all hosts from the database
func (s *Server) getHosts() ([]templates.Host, error) {
	rows, err := s.db.Query(`
		SELECT id, hostname, host_type, agent_version, os_version, 
		       nixpkgs_version, generation, last_seen, status, pending_command, 
		       theme_color, metrics_json, location, device_type, test_progress,
		       repo_url, repo_dir
		FROM hosts ORDER BY hostname
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var hosts []templates.Host
	for rows.Next() {
		var h struct {
			ID, Hostname, HostType                              string
			AgentVersion, OSVersion, NixpkgsVersion, Generation *string
			LastSeen                                            *string
			Status                                              string
			PendingCommand, ThemeColor, MetricsJSON             *string
			Location, DeviceType, TestProgressJSON              *string
			RepoURL, RepoDir                                    *string
		}
		if err := rows.Scan(&h.ID, &h.Hostname, &h.HostType, &h.AgentVersion,
			&h.OSVersion, &h.NixpkgsVersion, &h.Generation, &h.LastSeen,
			&h.Status, &h.PendingCommand, &h.ThemeColor, &h.MetricsJSON,
			&h.Location, &h.DeviceType, &h.TestProgressJSON,
			&h.RepoURL, &h.RepoDir); err != nil {
			s.log.Debug().Err(err).Msg("failed to scan host row")
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
		if h.MetricsJSON != nil {
			var metrics templates.Metrics
			if err := json.Unmarshal([]byte(*h.MetricsJSON), &metrics); err == nil {
				host.Metrics = &metrics
			}
		}
		if h.Location != nil {
			host.Location = *h.Location
		} else {
			host.Location = "home"
		}
		if h.DeviceType != nil {
			host.DeviceType = *h.DeviceType
		} else {
			host.DeviceType = "desktop"
		}
		if h.RepoURL != nil {
			host.RepoURL = *h.RepoURL
		}
		if h.RepoDir != nil {
			host.RepoDir = *h.RepoDir
		}
		if h.TestProgressJSON != nil {
			var testProgress templates.TestProgress
			if err := json.Unmarshal([]byte(*h.TestProgressJSON), &testProgress); err == nil {
				host.TestProgress = &testProgress
			}
		}

		// Populate Update Status (P5000 - Git check)
		host.UpdateStatus = s.getUpdateStatus(host.Generation, host.RepoURL, host.RepoDir)

		hosts = append(hosts, host)
	}
	return hosts, nil
}

// getUpdateStatus returns the update status for a host based on its generation.
func (s *Server) getUpdateStatus(generation, repoURL, repoDir string) *templates.UpdateStatus {
	status := &templates.UpdateStatus{
		RepoURL: repoURL,
		RepoDir: repoDir,
	}

	// Git status (from GitHub Pages)
	if s.versionFetcher != nil {
		gitStatus, gitMsg, gitChecked := s.versionFetcher.GetGitStatus(generation)
		status.Git = templates.StatusCheck{
			Status:    gitStatus,
			Message:   gitMsg,
			CheckedAt: gitChecked,
		}
	} else {
		status.Git = templates.StatusCheck{
			Status:    "unknown",
			Message:   "Version tracking not configured",
			CheckedAt: "",
		}
	}

	// Lock status (placeholder - requires agent-side implementation)
	status.Lock = templates.StatusCheck{
		Status:    "unknown",
		Message:   "Lock status requires agent update",
		CheckedAt: "",
	}

	// System status (placeholder - requires agent-side implementation)
	status.System = templates.StatusCheck{
		Status:    "unknown",
		Message:   "System status requires agent update",
		CheckedAt: "",
	}

	return status
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

	// Upgrade connection with origin validation
	conn, err := s.upgrader().Upgrade(w, r, nil)
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

// handleAddHost manually adds a host to the database.
func (s *Server) handleAddHost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hostname   string `json:"hostname"`
		HostType   string `json:"host_type"`
		Location   string `json:"location"`
		DeviceType string `json:"device_type"`
		ThemeColor string `json:"theme_color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Validate hostname
	if req.Hostname == "" {
		http.Error(w, "Hostname required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.HostType == "" {
		req.HostType = "nixos"
	}
	if req.Location == "" {
		req.Location = "home"
	}
	if req.DeviceType == "" {
		req.DeviceType = "desktop"
	}
	if req.ThemeColor == "" {
		req.ThemeColor = "#7aa2f7"
	}

	// Insert host
	hostID := req.Hostname
	_, err := s.db.Exec(`
		INSERT INTO hosts (id, hostname, host_type, status, location, device_type, theme_color)
		VALUES (?, ?, ?, 'offline', ?, ?, ?)
		ON CONFLICT(hostname) DO UPDATE SET
			host_type = excluded.host_type,
			location = excluded.location,
			device_type = excluded.device_type,
			theme_color = excluded.theme_color
	`, hostID, req.Hostname, req.HostType, req.Location, req.DeviceType, req.ThemeColor)
	if err != nil {
		s.log.Error().Err(err).Str("hostname", req.Hostname).Msg("failed to add host")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	s.log.Info().Str("host_id", hostID).Msg("host added manually")

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "created", "host_id": hostID})
}

// handleDeleteHost removes a host from the database.
func (s *Server) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "hostID")

	// Check if host is online - don't allow deleting online hosts
	agent := s.hub.GetAgent(hostID)
	if agent != nil {
		http.Error(w, "Cannot delete online host", http.StatusConflict)
		return
	}

	// Delete command logs first (foreign key)
	_, _ = s.db.Exec(`DELETE FROM command_logs WHERE host_id = ?`, hostID)

	// Delete the host
	result, err := s.db.Exec(`DELETE FROM hosts WHERE id = ?`, hostID)
	if err != nil {
		s.log.Error().Err(err).Str("host_id", hostID).Msg("failed to delete host")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "Host not found", http.StatusNotFound)
		return
	}

	s.log.Info().Str("host_id", hostID).Msg("host deleted")

	// Broadcast to browsers
	s.hub.BroadcastToBrowsers(map[string]any{
		"type": "host_deleted",
		"payload": map[string]any{
			"host_id": hostID,
		},
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "deleted", "host_id": hostID})
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

