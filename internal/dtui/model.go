// Package dtui provides the daemon manager terminal user interface.
package dtui

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flashingpumpkin/orbital/internal/daemon"
)

// View represents the current view state.
type View int

const (
	ViewManager View = iota
	ViewSession
	ViewChat
	ViewNewSession
	ViewHelp
)

// Model is the main TUI model.
type Model struct {
	client     *daemon.Client
	projectDir string

	// Current view
	view View

	// Session list
	sessions       []*daemon.Session
	selectedIdx    int
	expandedGroups map[string]bool

	// Session view
	currentSession *daemon.Session
	outputBuffer   []daemon.OutputMsg
	viewport       viewport.Model
	following      bool

	// Chat view
	chatInput   textinput.Model
	chatHistory []chatMessage
	chatViewport viewport.Model

	// New session dialog
	newSessionInput textinput.Model
	newSessionOpts  newSessionOptions

	// UI components
	spinner spinner.Model
	help    help.Model
	keys    keyMap

	// Window size
	width  int
	height int

	// State
	loading bool
	err     error
	quitting bool

	// Stream subscription
	streamCancel context.CancelFunc
}

type chatMessage struct {
	Role    string // "user" or "assistant"
	Content string
}

type newSessionOptions struct {
	specFile     string
	worktree     bool
	budget       float64
}

// keyMap defines key bindings.
type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Enter    key.Binding
	Back     key.Binding
	Stop     key.Binding
	Merge    key.Binding
	Chat     key.Binding
	Resume   key.Binding
	New      key.Binding
	Follow   key.Binding
	Quit     key.Binding
	Help     key.Binding
	ForceQuit key.Binding
}

var defaultKeyMap = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "collapse"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "expand"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "b"),
		key.WithHelp("esc/b", "back"),
	),
	Stop: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "stop"),
	),
	Merge: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "merge"),
	),
	Chat: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "chat"),
	),
	Resume: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "resume"),
	),
	New: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "new"),
	),
	Follow: key.NewBinding(
		key.WithKeys("f", "end"),
		key.WithHelp("f", "follow"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	ForceQuit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	),
}

// NewModel creates a new TUI model.
func NewModel(client *daemon.Client, projectDir string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 500

	newTi := textinput.New()
	newTi.Placeholder = "spec-file.md"
	newTi.CharLimit = 256

	return Model{
		client:         client,
		projectDir:     projectDir,
		view:           ViewManager,
		expandedGroups: map[string]bool{"running": true, "completed": false, "failed": false, "stopped": false},
		following:      true,
		spinner:        s,
		help:           help.New(),
		keys:           defaultKeyMap,
		chatInput:      ti,
		newSessionInput: newTi,
		newSessionOpts: newSessionOptions{budget: 50.0},
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchSessions,
		tickCmd(),
	)
}

// tickMsg is sent periodically to refresh data.
type tickMsg time.Time

// sessionsMsg contains fetched sessions.
type sessionsMsg struct {
	sessions []*daemon.Session
	err      error
}

// outputMsg contains new output for a session.
type outputMsg struct {
	msg daemon.OutputMsg
}

// chatResponseMsg contains a chat response.
type chatResponseMsg struct {
	response string
	err      error
}

// tickCmd returns a command that sends a tick every 500ms.
func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// fetchSessions fetches all sessions from the daemon.
func (m Model) fetchSessions() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sessions, err := m.client.ListSessions(ctx)
	return sessionsMsg{sessions: sessions, err: err}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport = viewport.New(msg.Width-4, msg.Height-10)
		m.chatViewport = viewport.New(msg.Width-4, msg.Height-14)
		m.help.Width = msg.Width

	case tea.KeyMsg:
		// Handle force quit
		if key.Matches(msg, m.keys.ForceQuit) {
			m.quitting = true
			return m, tea.Quit
		}

		// Handle view-specific keys
		switch m.view {
		case ViewManager:
			return m.updateManager(msg)
		case ViewSession:
			return m.updateSession(msg)
		case ViewChat:
			return m.updateChat(msg)
		case ViewNewSession:
			return m.updateNewSession(msg)
		case ViewHelp:
			if key.Matches(msg, m.keys.Back) || key.Matches(msg, m.keys.Quit) || key.Matches(msg, m.keys.Help) {
				m.view = ViewManager
			}
		}

	case tickMsg:
		cmds = append(cmds, m.fetchSessions, tickCmd())

	case sessionsMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.sessions = msg.sessions
			m.err = nil
		}

	case outputMsg:
		m.outputBuffer = append(m.outputBuffer, msg.msg)
		if m.following {
			m.viewport.GotoBottom()
		}

	case streamStartedMsg:
		// Cancel any previous stream
		if m.streamCancel != nil {
			m.streamCancel()
		}
		m.streamCancel = msg.cancel
		// Start waiting for output
		cmds = append(cmds, waitForOutput(msg.msgCh))

	case streamOutputMsg:
		m.outputBuffer = append(m.outputBuffer, msg.msg)
		m.viewport.SetContent(m.renderOutputBuffer())
		if m.following {
			m.viewport.GotoBottom()
		}
		// Continue waiting for more output
		cmds = append(cmds, waitForOutput(msg.msgCh))

	case streamEndedMsg:
		// Stream ended, no more output
		m.streamCancel = nil

	case chatResponseMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.chatHistory = append(m.chatHistory, chatMessage{
				Role:    "assistant",
				Content: msg.response,
			})
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// updateManager handles keys in manager view.
func (m Model) updateManager(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.getVisibleItems()

	switch {
	case key.Matches(msg, m.keys.Quit):
		m.quitting = true
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.view = ViewHelp

	case key.Matches(msg, m.keys.Up):
		if m.selectedIdx > 0 {
			m.selectedIdx--
		}

	case key.Matches(msg, m.keys.Down):
		if m.selectedIdx < len(items)-1 {
			m.selectedIdx++
		}

	case key.Matches(msg, m.keys.Left):
		if m.selectedIdx < len(items) {
			item := items[m.selectedIdx]
			if item.isGroup {
				m.expandedGroups[item.group] = false
			}
		}

	case key.Matches(msg, m.keys.Right):
		if m.selectedIdx < len(items) {
			item := items[m.selectedIdx]
			if item.isGroup {
				m.expandedGroups[item.group] = true
			}
		}

	case key.Matches(msg, m.keys.Enter):
		if m.selectedIdx < len(items) {
			item := items[m.selectedIdx]
			if item.isGroup {
				m.expandedGroups[item.group] = !m.expandedGroups[item.group]
			} else if item.session != nil {
				m.currentSession = item.session
				m.outputBuffer = nil
				m.view = ViewSession
				m.following = true
				return m, m.startOutputStream()
			}
		}

	case key.Matches(msg, m.keys.Stop):
		if m.selectedIdx < len(items) {
			item := items[m.selectedIdx]
			if item.session != nil && item.session.Status == daemon.StatusRunning {
				return m, m.stopSession(item.session.ID)
			}
		}

	case key.Matches(msg, m.keys.Resume):
		if m.selectedIdx < len(items) {
			item := items[m.selectedIdx]
			if item.session != nil && (item.session.Status == daemon.StatusInterrupted || item.session.Status == daemon.StatusStopped) {
				return m, m.resumeSession(item.session.ID)
			}
		}

	case key.Matches(msg, m.keys.Merge):
		if m.selectedIdx < len(items) {
			item := items[m.selectedIdx]
			if item.session != nil && item.session.Worktree != nil &&
				(item.session.Status == daemon.StatusCompleted || item.session.Status == daemon.StatusStopped) {
				return m, m.triggerMerge(item.session.ID)
			}
		}

	case key.Matches(msg, m.keys.Chat):
		if m.selectedIdx < len(items) {
			item := items[m.selectedIdx]
			if item.session != nil {
				m.currentSession = item.session
				m.chatHistory = nil
				m.chatInput.Reset()
				m.chatInput.Focus()
				m.view = ViewChat
			}
		}

	case key.Matches(msg, m.keys.New):
		m.newSessionInput.Reset()
		m.newSessionInput.Focus()
		m.view = ViewNewSession
	}

	return m, nil
}

// updateSession handles keys in session view.
func (m Model) updateSession(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.cancelStream()
		m.view = ViewManager
		m.currentSession = nil

	case key.Matches(msg, m.keys.Quit):
		m.cancelStream()
		m.view = ViewManager
		m.currentSession = nil

	case key.Matches(msg, m.keys.Follow):
		m.following = true
		m.viewport.GotoBottom()

	case key.Matches(msg, m.keys.Stop):
		if m.currentSession != nil && m.currentSession.Status == daemon.StatusRunning {
			return m, m.stopSession(m.currentSession.ID)
		}

	case key.Matches(msg, m.keys.Merge):
		if m.currentSession != nil && m.currentSession.Worktree != nil &&
			(m.currentSession.Status == daemon.StatusCompleted || m.currentSession.Status == daemon.StatusStopped) {
			return m, m.triggerMerge(m.currentSession.ID)
		}

	case key.Matches(msg, m.keys.Chat):
		if m.currentSession != nil {
			m.chatHistory = nil
			m.chatInput.Reset()
			m.chatInput.Focus()
			m.view = ViewChat
		}

	case key.Matches(msg, m.keys.Up):
		m.following = false
		m.viewport.LineUp(1)

	case key.Matches(msg, m.keys.Down):
		m.viewport.LineDown(1)
		if m.viewport.AtBottom() {
			m.following = true
		}
	}

	return m, nil
}

// updateChat handles keys in chat view.
func (m Model) updateChat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.view = ViewSession
		m.chatInput.Blur()
		return m, nil

	case msg.Type == tea.KeyEnter:
		if m.chatInput.Value() != "" && !m.loading {
			message := m.chatInput.Value()
			m.chatHistory = append(m.chatHistory, chatMessage{
				Role:    "user",
				Content: message,
			})
			m.chatInput.Reset()
			m.loading = true
			return m, m.sendChat(message)
		}
	}

	// Update text input
	var cmd tea.Cmd
	m.chatInput, cmd = m.chatInput.Update(msg)
	return m, cmd
}

// updateNewSession handles keys in new session view.
func (m Model) updateNewSession(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.view = ViewManager
		m.newSessionInput.Blur()
		return m, nil

	case msg.Type == tea.KeyEnter:
		if m.newSessionInput.Value() != "" {
			specFile := m.newSessionInput.Value()
			m.view = ViewManager
			m.newSessionInput.Blur()
			return m, m.startNewSession(specFile)
		}

	case msg.Type == tea.KeyTab:
		m.newSessionOpts.worktree = !m.newSessionOpts.worktree
	}

	// Update text input
	var cmd tea.Cmd
	m.newSessionInput, cmd = m.newSessionInput.Update(msg)
	return m, cmd
}

// listItem represents an item in the session list.
type listItem struct {
	isGroup bool
	group   string
	session *daemon.Session
}

// getVisibleItems returns the list of visible items based on expanded state.
func (m Model) getVisibleItems() []listItem {
	var items []listItem

	// Group sessions by status
	groups := map[string][]*daemon.Session{
		"running":   {},
		"completed": {},
		"failed":    {},
		"stopped":   {},
	}

	for _, s := range m.sessions {
		switch s.Status {
		case daemon.StatusRunning, daemon.StatusMerging:
			groups["running"] = append(groups["running"], s)
		case daemon.StatusCompleted, daemon.StatusMerged:
			groups["completed"] = append(groups["completed"], s)
		case daemon.StatusFailed, daemon.StatusConflict:
			groups["failed"] = append(groups["failed"], s)
		case daemon.StatusStopped, daemon.StatusInterrupted:
			groups["stopped"] = append(groups["stopped"], s)
		}
	}

	// Sort sessions by start time (newest first)
	for _, sessions := range groups {
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].StartedAt.After(sessions[j].StartedAt)
		})
	}

	// Build items list
	groupOrder := []string{"running", "completed", "failed", "stopped"}
	for _, group := range groupOrder {
		sessions := groups[group]
		if len(sessions) == 0 {
			continue
		}

		items = append(items, listItem{isGroup: true, group: group})
		if m.expandedGroups[group] {
			for _, s := range sessions {
				items = append(items, listItem{session: s})
			}
		}
	}

	return items
}

// streamSubscription tracks an active output stream subscription.
type streamSubscription struct {
	sessionID string
	cancel    context.CancelFunc
	msgCh     chan daemon.OutputMsg
}

// Command functions

func (m Model) startOutputStream() tea.Cmd {
	return func() tea.Msg {
		if m.currentSession == nil {
			return nil
		}

		// Create a channel for streaming messages
		msgCh := make(chan daemon.OutputMsg, 100)
		ctx, cancel := context.WithCancel(context.Background())

		// Start streaming in background
		go func() {
			defer close(msgCh)
			defer cancel()

			err := m.client.StreamOutput(ctx, m.currentSession.ID, func(msg daemon.OutputMsg) {
				select {
				case msgCh <- msg:
				case <-ctx.Done():
				}
			})
			if err != nil && ctx.Err() == nil {
				msgCh <- daemon.OutputMsg{Type: "error", Content: err.Error()}
			}
		}()

		return streamStartedMsg{
			sessionID: m.currentSession.ID,
			msgCh:     msgCh,
			cancel:    cancel,
		}
	}
}

// streamStartedMsg indicates streaming has started.
type streamStartedMsg struct {
	sessionID string
	msgCh     chan daemon.OutputMsg
	cancel    context.CancelFunc
}

// waitForOutput returns a command that waits for the next output message.
func waitForOutput(msgCh chan daemon.OutputMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-msgCh
		if !ok {
			return streamEndedMsg{}
		}
		return streamOutputMsg{msg: msg, msgCh: msgCh}
	}
}

// streamOutputMsg contains output from the stream.
type streamOutputMsg struct {
	msg   daemon.OutputMsg
	msgCh chan daemon.OutputMsg
}

// streamEndedMsg indicates the stream has ended.
type streamEndedMsg struct{}

func (m Model) stopSession(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.client.StopSession(ctx, id)
		return nil
	}
}

func (m Model) resumeSession(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.client.ResumeSession(ctx, id)
		return nil
	}
}

func (m Model) triggerMerge(id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.client.TriggerMerge(ctx, id)
		return nil
	}
}

func (m Model) sendChat(message string) tea.Cmd {
	return func() tea.Msg {
		if m.currentSession == nil {
			return chatResponseMsg{err: fmt.Errorf("no session selected")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		response, err := m.client.SendChat(ctx, m.currentSession.ID, message)
		return chatResponseMsg{response: response, err: err}
	}
}

func (m Model) startNewSession(specFile string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req := daemon.StartSessionRequest{
			SpecFiles:     []string{specFile},
			Worktree:      m.newSessionOpts.worktree,
			Budget:        m.newSessionOpts.budget,
			MaxIterations: 50,
		}

		_, err := m.client.StartSession(ctx, req)
		if err != nil {
			return sessionsMsg{err: err}
		}
		return nil
	}
}

// cancelStream cancels the current output stream subscription.
func (m *Model) cancelStream() {
	if m.streamCancel != nil {
		m.streamCancel()
		m.streamCancel = nil
	}
}

// renderOutputBuffer renders the output buffer as a string for the viewport.
func (m Model) renderOutputBuffer() string {
	var lines []string
	for _, msg := range m.outputBuffer {
		prefix := ""
		switch msg.Type {
		case "error":
			prefix = "[ERR] "
		case "tool":
			prefix = "[TOOL] "
		case "status":
			prefix = "[STATUS] "
		case "stats":
			prefix = "[STATS] "
		}
		lines = append(lines, prefix+msg.Content)
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
