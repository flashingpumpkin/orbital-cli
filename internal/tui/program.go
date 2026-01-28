package tui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Program wraps the tea.Program and Bridge for lifecycle management.
type Program struct {
	program *tea.Program
	bridge  *Bridge
	tracker *TaskTracker
}

// New creates a new TUI program with the given initial session and progress.
// Returns the Program wrapper which provides access to both the tea.Program and Bridge.
// The theme parameter specifies the colour theme: "auto", "dark", or "light".
// If theme is "auto", it will be resolved using DetectTheme().
func New(session SessionInfo, progress ProgressInfo, theme string) *Program {
	// Handle NO_COLOR environment variable
	if os.Getenv("NO_COLOR") != "" {
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	// Resolve theme
	resolvedTheme := ResolveTheme(Theme(theme))

	// Create the model with initial values and resolved theme
	model := NewModelWithTheme(resolvedTheme)
	model.session = session
	model.tabs = model.buildTabs()
	model.progress = progress

	// Create task tracker
	tracker := NewTaskTracker()

	// Create the tea program
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Create the bridge
	bridge := NewBridge(program, tracker)

	return &Program{
		program: program,
		bridge:  bridge,
		tracker: tracker,
	}
}

// Run starts the TUI program. This blocks until the program exits.
func (p *Program) Run() error {
	_, err := p.program.Run()
	return err
}

// Bridge returns the Bridge which implements io.Writer for streaming output.
func (p *Program) Bridge() *Bridge {
	return p.bridge
}

// Send sends a message to the program.
func (p *Program) Send(msg tea.Msg) {
	p.program.Send(msg)
}

// Quit sends a quit message to the program.
func (p *Program) Quit() {
	p.program.Quit()
}

// SendProgress sends a progress update to the program.
func (p *Program) SendProgress(progress ProgressInfo) {
	p.program.Send(ProgressMsg(progress))
}

// SendSession sends session info to the program.
func (p *Program) SendSession(session SessionInfo) {
	p.program.Send(SessionMsg(session))
}

// SendOutput sends a formatted output line to the program.
func (p *Program) SendOutput(line string) {
	p.program.Send(OutputLineMsg(line))
}

// Kill forcefully terminates the program.
func (p *Program) Kill() {
	p.program.Kill()
}

// Wait waits for the program to finish.
func (p *Program) Wait() {
	p.program.Wait()
}

// Close cleans up resources including the Bridge's message pump goroutine.
// Should be called after the program exits.
func (p *Program) Close() {
	if p.bridge != nil {
		p.bridge.Close()
	}
}

// SendInitialPrompt formats and sends the initial prompt to the TUI viewport.
// This should be called after the TUI is ready (after the startup delay).
func (p *Program) SendInitialPrompt(prompt string) {
	// Send header with blank line after
	p.program.Send(OutputLineMsg("📋 Initial Prompt"))
	p.program.Send(OutputLineMsg(""))

	// Send the prompt content (padding applied globally in syncViewportContent)
	p.program.Send(OutputLineMsg(prompt))

	// Blank line after prompt
	p.program.Send(OutputLineMsg(""))
}

