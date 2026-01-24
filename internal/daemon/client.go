package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Client is a client for the orbital daemon.
type Client struct {
	socketPath string
	httpClient *http.Client
}

// NewClient creates a new daemon client.
func NewClient(projectDir string) *Client {
	socketPath := SocketPath(projectDir)

	return &Client{
		socketPath: socketPath,
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
			Timeout: 30 * time.Second,
		},
	}
}

// IsRunning checks if the daemon is running.
func (c *Client) IsRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "http://unix/health", nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Health checks the daemon health.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://unix/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("daemon not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// Status returns the daemon status.
func (c *Client) Status(ctx context.Context) (*DaemonStatus, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://unix/status", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var status DaemonStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

// ListSessions returns all sessions.
func (c *Client) ListSessions(ctx context.Context) ([]*Session, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://unix/sessions", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var listResp SessionListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, err
	}

	return listResp.Sessions, nil
}

// GetSession returns a session by ID.
func (c *Client) GetSession(ctx context.Context, id string) (*Session, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://unix/sessions/%s", id), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("session not found")
	}

	var sessResp SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessResp); err != nil {
		return nil, err
	}

	if sessResp.Error != "" {
		return nil, fmt.Errorf("session error: %s", sessResp.Error)
	}

	return sessResp.Session, nil
}

// StartSession starts a new session.
func (c *Client) StartSession(ctx context.Context, req StartSessionRequest) (*Session, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "http://unix/sessions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to start session: %s", string(bodyBytes))
	}

	var sessResp SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessResp); err != nil {
		return nil, err
	}

	return sessResp.Session, nil
}

// StopSession stops a running session.
func (c *Client) StopSession(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("http://unix/sessions/%s", id), nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to stop session: %s", string(bodyBytes))
	}

	return nil
}

// ResumeSession resumes an interrupted or stopped session.
func (c *Client) ResumeSession(ctx context.Context, id string) (*Session, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("http://unix/sessions/%s/resume", id), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to resume session: %s", string(bodyBytes))
	}

	var sessResp SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessResp); err != nil {
		return nil, err
	}

	return sessResp.Session, nil
}

// TriggerMerge triggers a worktree merge for a session.
func (c *Client) TriggerMerge(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("http://unix/sessions/%s/merge", id), nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to trigger merge: %s", string(bodyBytes))
	}

	return nil
}

// StreamOutput streams session output via SSE.
func (c *Client) StreamOutput(ctx context.Context, id string, handler func(OutputMsg)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://unix/sessions/%s/output", id), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	// Use a client without timeout for streaming
	streamClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", c.socketPath)
			},
		},
	}

	resp, err := streamClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to stream output: %s", string(bodyBytes))
	}

	// Read SSE events
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var msg OutputMsg
			if err := json.Unmarshal([]byte(data), &msg); err == nil {
				handler(msg)
			}
		}
	}

	return scanner.Err()
}

// SendChat sends a chat message.
func (c *Client) SendChat(ctx context.Context, sessionID string, message string) (string, error) {
	body, err := json.Marshal(ChatRequest{Message: message})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("http://unix/sessions/%s/chat", sessionID), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("chat failed: %s", string(bodyBytes))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}

	if chatResp.Error != "" {
		return "", fmt.Errorf("chat error: %s", chatResp.Error)
	}

	return chatResp.Response, nil
}

// Shutdown shuts down the daemon.
func (c *Client) Shutdown(ctx context.Context, force bool) error {
	url := "http://unix/shutdown"
	if force {
		url += "?force=true"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		return fmt.Errorf("%v running sessions - use --force to override", result["running_sessions"])
	}

	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("shutdown failed: %s", string(bodyBytes))
	}

	return nil
}

// IsDaemonRunning checks if a daemon is running for the project.
func IsDaemonRunning(projectDir string) bool {
	client := NewClient(projectDir)
	return client.IsRunning()
}

// GetDaemonPID returns the PID of the running daemon, or 0 if not running.
func GetDaemonPID(projectDir string) int {
	pidFile := PIDFile(projectDir)
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}

	return pid
}
