package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/DylanSharp/dtools/internal/ralph/domain"
	"github.com/DylanSharp/dtools/internal/ralph/service"
)

// Model is the Bubbletea model for the ralph TUI
type Model struct {
	// Project state
	project   *domain.Project
	projectID string
	events    []domain.ExecutionEvent

	// UI state
	statusBar    StatusBar
	width        int
	height       int
	scrollOffset int
	err          error
	streaming    bool
	complete     bool

	// Services
	service *service.ProjectService

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Channels
	eventsChan <-chan domain.ExecutionEvent
}

// NewModel creates a new Model for running a project
func NewModel(
	svc *service.ProjectService,
	projectID string,
) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	return &Model{
		events:    []domain.ExecutionEvent{},
		statusBar: NewStatusBar(),
		service:   svc,
		projectID: projectID,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.loadProjectCmd(),
		tickCmd(),
	)
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case ProjectLoadedMsg:
		m.project = msg.Project
		m.statusBar.Update(msg.Project)
		// Start execution
		return m, m.startExecutionCmd()

	case StreamStartedMsg:
		m.eventsChan = msg.Events
		m.streaming = true
		return m, m.readEventCmd()

	case ExecutionEventMsg:
		m.events = append(m.events, msg.Event)

		// Update status bar for story events
		if msg.Event.IsStoryEvent() || msg.Event.IsProjectEvent() {
			if m.project != nil {
				// Reload project to get updated state
				if updated, err := m.service.GetProject(m.projectID); err == nil {
					m.project = updated
					m.statusBar.Update(updated)
				}
			}
		}

		// Auto-scroll to bottom
		m.scrollToBottom()

		// Continue reading events
		if m.eventsChan != nil {
			return m, m.readEventCmd()
		}
		return m, nil

	case StreamEndedMsg:
		m.streaming = false
		m.eventsChan = nil
		m.complete = true
		// Final status update
		if updated, err := m.service.GetProject(m.projectID); err == nil {
			m.project = updated
			m.statusBar.Update(updated)
		}
		return m, nil

	case ErrorMsg:
		m.err = msg.Err
		m.statusBar.SetError(msg.Err)
		return m, nil

	case TickMsg:
		return m, tickCmd()

	case ProjectCompleteMsg:
		m.project = msg.Project
		m.statusBar.Update(msg.Project)
		m.complete = true
		return m, nil
	}

	return m, nil
}

// View renders the UI
func (m *Model) View() string {
	if m.err != nil {
		return RenderError(m.err, m.width)
	}
	return RenderView(m)
}

// handleKeyPress handles keyboard input
func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle error state - any key dismisses
	if m.err != nil {
		m.err = nil
		m.statusBar.ClearError()
		return m, nil
	}

	switch msg.String() {
	case "q", "Q", "ctrl+c":
		m.cancel()
		return m, tea.Quit

	case "up", "k":
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
		return m, nil

	case "down", "j":
		m.scrollOffset++
		return m, nil

	case "pgup":
		m.scrollOffset -= 10
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
		return m, nil

	case "pgdown":
		m.scrollOffset += 10
		return m, nil

	case "home", "g":
		m.scrollOffset = 0
		return m, nil

	case "end", "G":
		m.scrollToBottom()
		return m, nil

	case "r", "R":
		if !m.streaming && !m.complete {
			// Restart execution
			m.events = []domain.ExecutionEvent{}
			m.scrollOffset = 0
			return m, m.startExecutionCmd()
		}
		return m, nil
	}

	return m, nil
}

// scrollToBottom scrolls to show the latest content
func (m *Model) scrollToBottom() {
	viewHeight := m.height - statusBarHeight - helpHeight - headerHeight - 2
	if viewHeight < minViewportHeight {
		viewHeight = minViewportHeight
	}

	if len(m.events) > viewHeight {
		m.scrollOffset = len(m.events) - viewHeight
	} else {
		m.scrollOffset = 0
	}
}

// Commands

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

func (m *Model) loadProjectCmd() tea.Cmd {
	return func() tea.Msg {
		project, err := m.service.GetProject(m.projectID)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return ProjectLoadedMsg{Project: project}
	}
}

func (m *Model) startExecutionCmd() tea.Cmd {
	return func() tea.Msg {
		events, err := m.service.RunProject(m.ctx, m.projectID)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return StreamStartedMsg{Events: events}
	}
}

func (m *Model) readEventCmd() tea.Cmd {
	return func() tea.Msg {
		if m.eventsChan == nil {
			return StreamEndedMsg{}
		}

		select {
		case event, ok := <-m.eventsChan:
			if !ok {
				return StreamEndedMsg{}
			}
			return ExecutionEventMsg{Event: event}
		case <-m.ctx.Done():
			return StreamEndedMsg{}
		}
	}
}

// GetProject returns the current project
func (m *Model) GetProject() *domain.Project {
	return m.project
}

// IsComplete returns true if execution is complete
func (m *Model) IsComplete() bool {
	return m.complete
}

// IsStreaming returns true if currently streaming events
func (m *Model) IsStreaming() bool {
	return m.streaming
}

// StatusModel creates a simple model for displaying status (non-interactive)
type StatusModel struct {
	project *domain.Project
	width   int
	height  int
}

// NewStatusModel creates a new status display model
func NewStatusModel(project *domain.Project) *StatusModel {
	return &StatusModel{project: project}
}

// Init implements tea.Model
func (m *StatusModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m *StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		return m, tea.Quit
	}
	return m, nil
}

// View implements tea.Model
func (m *StatusModel) View() string {
	if m.project == nil {
		return "No project loaded"
	}

	var sections []string

	// Header
	header := headerStyle.Render(titleStyle.Render("Ralph - " + m.project.Name))
	sections = append(sections, header)

	// Progress summary
	summary := RenderProgressSummary(m.project)
	sections = append(sections, summary)

	// Story list
	storyList := RenderStoryList(m.project, "", m.width)
	sections = append(sections, storyList)

	// Help
	help := helpStyle.Render("Press any key to exit")
	sections = append(sections, help)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
