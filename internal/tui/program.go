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
func New(session SessionInfo, progress ProgressInfo) *Program {
	// Handle NO_COLOR environment variable
	if os.Getenv("NO_COLOR") != "" {
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	// Create the model with initial values
	model := NewModel()
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
