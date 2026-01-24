package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Server is the orbital daemon HTTP server.
type Server struct {
	projectDir   string
	socketPath   string
	pidFile      string
	registry     *Registry
	config       *DaemonConfig
	httpServer   *http.Server
	listener     net.Listener
	runner       *SessionRunner
	startedAt    time.Time
	shutdownCh   chan struct{} // Closed to trigger shutdown
	shutdownMu   sync.Mutex
	shuttingDown bool
}

// NewServer creates a new daemon server.
func NewServer(projectDir string, cfg *DaemonConfig) *Server {
	if cfg == nil {
		cfg = DefaultDaemonConfig()
	}

	s := &Server{
		projectDir: projectDir,
		socketPath: filepath.Join(projectDir, ".orbital", "daemon.sock"),
		pidFile:    filepath.Join(projectDir, ".orbital", "daemon.pid"),
		registry:   NewRegistry(projectDir),
		config:     cfg,
		startedAt:  time.Now(),
		shutdownCh: make(chan struct{}),
	}

	s.runner = NewSessionRunner(s.registry, projectDir, cfg)
	return s
}

// SocketPath returns the path to the Unix socket.
func SocketPath(projectDir string) string {
	return filepath.Join(projectDir, ".orbital", "daemon.sock")
}

// PIDFile returns the path to the PID file.
func PIDFile(projectDir string) string {
	return filepath.Join(projectDir, ".orbital", "daemon.pid")
}

// Start starts the daemon server.
func (s *Server) Start(ctx context.Context) error {
	// Create .orbital directory if needed
	orbitalDir := filepath.Join(s.projectDir, ".orbital")
	if err := os.MkdirAll(orbitalDir, 0755); err != nil {
		return fmt.Errorf("failed to create .orbital directory: %w", err)
	}

	// Remove stale socket file
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove stale socket: %w", err)
	}

	// Load existing state
	if err := s.registry.Load(); err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}
	s.listener = listener

	// Make socket accessible
	if err := os.Chmod(s.socketPath, 0660); err != nil {
		listener.Close()
		return fmt.Errorf("failed to chmod socket: %w", err)
	}

	// Write PID file
	if err := s.writePIDFile(); err != nil {
		listener.Close()
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Create HTTP server with routes
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // No timeout for SSE
		IdleTimeout:  120 * time.Second,
	}

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for shutdown signal or context cancellation
	select {
	case <-ctx.Done():
	case sig := <-sigCh:
		fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
	case <-s.shutdownCh:
		fmt.Printf("\nShutdown requested via API, shutting down...\n")
	case err := <-errCh:
		return err
	}

	return s.Shutdown()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	// Stop all running sessions
	for _, session := range s.registry.ListByStatus(StatusRunning) {
		s.runner.Stop(session.ID)
	}

	// Shutdown HTTP server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	// Remove socket and PID file
	os.Remove(s.socketPath)
	os.Remove(s.pidFile)

	return nil
}

// writePIDFile writes the current process ID to the PID file.
func (s *Server) writePIDFile() error {
	pid := os.Getpid()
	return os.WriteFile(s.pidFile, []byte(strconv.Itoa(pid)), 0644)
}

// registerRoutes sets up the HTTP routes.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/sessions", s.handleSessions)
	mux.HandleFunc("/sessions/", s.handleSession)
	mux.HandleFunc("/shutdown", s.handleShutdown)
}

// handleHealth handles GET /health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleStatus handles GET /status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := DaemonStatus{
		PID:        os.Getpid(),
		StartedAt:  s.startedAt,
		ProjectDir: s.projectDir,
		Sessions:   s.registry.Count(),
		TotalCost:  s.registry.TotalCost(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleSessions handles /sessions
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listSessions(w, r)
	case http.MethodPost:
		s.startSession(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// listSessions handles GET /sessions
func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.registry.List()
	resp := SessionListResponse{
		Sessions: sessions,
		Total:    len(sessions),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// maxRequestBodySize is the maximum allowed request body size (1MB).
const maxRequestBodySize = 1 << 20 // 1MB

// startSession handles POST /sessions
func (s *Server) startSession(w http.ResponseWriter, r *http.Request) {
	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	var req StartSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if len(req.SpecFiles) == 0 {
		http.Error(w, "spec_files is required", http.StatusBadRequest)
		return
	}

	// Apply defaults
	if req.Budget <= 0 {
		req.Budget = s.config.DefaultBudget
	}
	if req.MaxIterations <= 0 {
		req.MaxIterations = 50
	}
	if req.Model == "" {
		req.Model = "opus"
	}
	if req.CheckerModel == "" {
		req.CheckerModel = "haiku"
	}
	if req.Workflow == "" {
		req.Workflow = s.config.DefaultWorkflow
	}

	// Start session
	session, err := s.runner.Start(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to start session: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(SessionResponse{Session: session})
}

// isValidSessionID checks if a session ID is valid (alphanumeric only).
func isValidSessionID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

// handleSession handles /sessions/{id}/*
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	// Parse session ID and action from path
	path := r.URL.Path[len("/sessions/"):]
	var sessionID, action string

	if idx := indexOf(path, '/'); idx != -1 {
		sessionID = path[:idx]
		action = path[idx+1:]
	} else {
		sessionID = path
		action = ""
	}

	// Validate session ID to prevent path traversal
	if !isValidSessionID(sessionID) {
		http.Error(w, "invalid session ID", http.StatusBadRequest)
		return
	}

	// Check session exists
	session, exists := s.registry.Get(sessionID)
	if !exists {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	switch {
	case action == "" && r.Method == http.MethodGet:
		s.getSession(w, session)
	case action == "" && r.Method == http.MethodDelete:
		s.stopSession(w, r, sessionID)
	case action == "output" && r.Method == http.MethodGet:
		s.streamOutput(w, r, sessionID)
	case action == "merge" && r.Method == http.MethodPost:
		s.triggerMerge(w, r, sessionID)
	case action == "chat" && r.Method == http.MethodPost:
		s.sendChat(w, r, session)
	case action == "chat" && r.Method == http.MethodGet:
		s.streamChat(w, r, session)
	case action == "resume" && r.Method == http.MethodPost:
		s.resumeSession(w, r, sessionID)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// getSession handles GET /sessions/{id}
func (s *Server) getSession(w http.ResponseWriter, session *Session) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SessionResponse{Session: session})
}

// stopSession handles DELETE /sessions/{id}
func (s *Server) stopSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	if err := s.runner.Stop(sessionID); err != nil {
		http.Error(w, fmt.Sprintf("failed to stop session: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// streamOutput handles GET /sessions/{id}/output (SSE)
func (s *Server) streamOutput(w http.ResponseWriter, r *http.Request, sessionID string) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Subscribe to output
	ch, history, done, err := s.registry.Subscribe(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer s.registry.Unsubscribe(sessionID, ch)

	// Send history first
	for _, msg := range history {
		data, _ := json.Marshal(msg)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()

	// Stream new messages
	for {
		select {
		case <-r.Context().Done():
			return
		case <-done:
			// Session completed, close connection
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(msg)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// triggerMerge handles POST /sessions/{id}/merge
func (s *Server) triggerMerge(w http.ResponseWriter, r *http.Request, sessionID string) {
	session, exists := s.registry.Get(sessionID)
	if !exists {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	if session.Worktree == nil {
		http.Error(w, "session has no worktree", http.StatusBadRequest)
		return
	}

	if session.Status != StatusCompleted && session.Status != StatusStopped {
		http.Error(w, "session must be completed or stopped to merge", http.StatusBadRequest)
		return
	}

	// Start merge in background and wait briefly for initial errors
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.runner.Merge(sessionID)
	}()

	// Wait up to 500ms for immediate errors (lock conflicts, validation)
	select {
	case err := <-errCh:
		if err != nil {
			http.Error(w, fmt.Sprintf("merge failed: %v", err), http.StatusInternalServerError)
			return
		}
		// Merge completed successfully (fast merge)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "merged"})
	case <-time.After(500 * time.Millisecond):
		// Merge is running in background
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "merging"})
	}
}

// sendChat handles POST /sessions/{id}/chat
func (s *Server) sendChat(w http.ResponseWriter, r *http.Request, session *Session) {
	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	response, err := s.runner.Chat(r.Context(), session, req.Message)
	if err != nil {
		http.Error(w, fmt.Sprintf("chat error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ChatResponse{Response: response})
}

// streamChat handles GET /sessions/{id}/chat (SSE for chat responses)
func (s *Server) streamChat(w http.ResponseWriter, r *http.Request, session *Session) {
	// For now, return not implemented
	// Full chat streaming would require maintaining chat session state
	http.Error(w, "chat streaming not yet implemented", http.StatusNotImplemented)
}

// resumeSession handles POST /sessions/{id}/resume
func (s *Server) resumeSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	session, exists := s.registry.Get(sessionID)
	if !exists {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	if session.Status != StatusInterrupted && session.Status != StatusStopped {
		http.Error(w, "session must be interrupted or stopped to resume", http.StatusBadRequest)
		return
	}

	if err := s.runner.Resume(r.Context(), sessionID); err != nil {
		http.Error(w, fmt.Sprintf("failed to resume session: %v", err), http.StatusInternalServerError)
		return
	}

	// Refresh session data
	session, _ = s.registry.Get(sessionID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SessionResponse{Session: session})
}

// handleShutdown handles POST /shutdown
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for force flag
	force := r.URL.Query().Get("force") == "true"

	// Check if there are running sessions
	running := s.registry.ListByStatus(StatusRunning)
	if len(running) > 0 && !force {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":            "sessions are still running",
			"running_sessions": len(running),
			"hint":             "use ?force=true to force shutdown",
		})
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "shutting down"})

	// Trigger shutdown in background (platform-independent)
	go func() {
		time.Sleep(100 * time.Millisecond) // Allow response to be sent
		s.triggerShutdown()
	}()
}

// triggerShutdown signals the server to shut down.
func (s *Server) triggerShutdown() {
	s.shutdownMu.Lock()
	defer s.shutdownMu.Unlock()

	if !s.shuttingDown {
		s.shuttingDown = true
		close(s.shutdownCh)
	}
}

// indexOf returns the index of the first occurrence of sep in s, or -1 if not found.
func indexOf(s string, sep byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return i
		}
	}
	return -1
}
