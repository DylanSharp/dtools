package ui

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
	"github.com/DylanSharp/dtools/internal/coderabbit/service"
)

// Model is the Bubbletea model for the review TUI
type Model struct {
	// Review state
	review   *domain.Review
	thoughts []domain.ThoughtChunk

	// UI state
	statusBar     StatusBar
	width         int
	height        int
	scrollOffset  int
	err           error

	// Mode flags
	watchMode      bool
	confirmingExit bool
	streaming      bool
	satisfied      bool
	complete       bool  // Review finished (with or without comments)
	fetching       bool  // Currently fetching data from GitHub

	// Services
	reviewService *service.ReviewService
	watcher       *service.Watcher

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Channels
	thoughtsChan <-chan domain.ThoughtChunk
	watchChan    <-chan service.WatchEvent

	// Config
	config service.ReviewConfig
}

// NewModel creates a new Model for a single review
func NewModel(
	reviewService *service.ReviewService,
	config service.ReviewConfig,
) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	return &Model{
		thoughts:      []domain.ThoughtChunk{},
		statusBar:     NewStatusBar(),
		reviewService: reviewService,
		ctx:           ctx,
		cancel:        cancel,
		config:        config,
		watchMode:     false,
	}
}

// NewWatchModel creates a new Model for watch mode
func NewWatchModel(
	reviewService *service.ReviewService,
	config service.ReviewConfig,
	watchOpts service.WatchOptions,
) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	watcher := service.NewWatcher(reviewService, watchOpts)

	return &Model{
		thoughts:      []domain.ThoughtChunk{},
		statusBar:     NewStatusBar(),
		reviewService: reviewService,
		watcher:       watcher,
		ctx:           ctx,
		cancel:        cancel,
		config:        config,
		watchMode:     true,
	}
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.EnterAltScreen,
		tickCmd(),
	}

	if m.watchMode {
		// Start watch mode
		cmds = append(cmds, m.startWatchCmd())
	} else {
		// Start single review
		cmds = append(cmds, m.startReviewCmd())
	}

	return tea.Batch(cmds...)
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

	case ThoughtMsg:
		m.thoughts = append(m.thoughts, msg.Thought)
		m.statusBar.CommentsProcessed++
		m.statusBar.CurrentFile = msg.Thought.File

		// Auto-scroll to bottom
		m.scrollToBottom()

		// Continue reading thoughts
		if m.thoughtsChan != nil {
			return m, m.readThoughtCmd()
		}
		return m, nil

	case ReviewStartedMsg:
		m.review = msg.Review
		m.statusBar.Update(msg.Review)
		m.thoughtsChan = msg.Thoughts
		m.streaming = true
		m.fetching = false
		m.complete = false

		// Prepend comments being addressed so user can see them
		m.thoughts = m.buildCommentSummary(msg.Review)

		// Start reading thoughts
		return m, m.readThoughtCmd()

	case ReviewCompleteMsg:
		m.review = msg.Review
		m.statusBar.Update(msg.Review)
		m.streaming = false
		m.fetching = false
		m.complete = true
		m.thoughtsChan = nil
		if msg.Review != nil && msg.Review.Satisfied {
			m.satisfied = true
		}
		return m, nil

	case WatchEventMsg:
		return m.handleWatchEvent(msg.Event)

	case ErrorMsg:
		m.err = msg.Err
		m.statusBar.SetError(msg.Err)
		return m, nil

	case TickMsg:
		// Update cooldown/batch wait remaining
		if m.watcher != nil {
			cooldown := m.watcher.GetCooldownRemaining()
			batchWait := m.watcher.GetBatchWaitRemaining()
			m.statusBar.SetWatchState(m.watcher.GetState(), cooldown, batchWait)
		}
		return m, tickCmd()

	case ManualConfirmMsg:
		m.confirmingExit = false
		if msg.Confirmed {
			if m.watcher != nil {
				m.watcher.ConfirmSatisfied()
			}
			return m, tea.Quit
		}
		if m.watcher != nil {
			m.watcher.RejectSatisfied()
		}
		return m, nil
	}

	return m, nil
}

// View renders the UI
func (m *Model) View() string {
	// Show error dialog if there's an error
	if m.err != nil {
		return RenderError(m.err, m.width)
	}

	// Show confirmation dialog
	if m.confirmingExit {
		return RenderConfirmDialog(m.width)
	}

	return RenderView(m)
}

// handleKeyPress handles keyboard input
func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle confirmation dialog
	if m.confirmingExit {
		switch msg.String() {
		case "y", "Y":
			return m, func() tea.Msg {
				return ManualConfirmMsg{Confirmed: true}
			}
		case "n", "N":
			return m, func() tea.Msg {
				return ManualConfirmMsg{Confirmed: false}
			}
		}
		return m, nil
	}

	// Handle error state
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
		if !m.watchMode && !m.streaming {
			// Refresh - restart review
			m.thoughts = []domain.ThoughtChunk{}
			m.scrollOffset = 0
			return m, m.startReviewCmd()
		}
		return m, nil

	case "o", "O":
		// Open PR in GitHub
		if m.config.PRNumber > 0 {
			return m, m.openPRCmd()
		}
		return m, nil
	}

	return m, nil
}

// handleWatchEvent handles watch mode events
func (m *Model) handleWatchEvent(event service.WatchEvent) (tea.Model, tea.Cmd) {
	m.statusBar.SetWatchState(m.watcher.GetState(), m.watcher.GetCooldownRemaining(), m.watcher.GetBatchWaitRemaining())

	switch event.Type {
	case service.WatchEventNewComments, service.WatchEventNewCIFailures:
		m.review = event.Review
		m.statusBar.Update(event.Review)
		m.thoughtsChan = event.Thoughts
		m.streaming = true
		// Clear previous thoughts for new review iteration
		m.thoughts = []domain.ThoughtChunk{}
		// Read both thoughts and continue watching for more events
		return m, tea.Batch(m.readThoughtCmd(), m.readWatchEventCmd())

	case service.WatchEventReviewComplete:
		m.review = event.Review
		m.statusBar.Update(event.Review)
		m.streaming = false
		m.thoughtsChan = nil
		// Continue watching for more events
		return m, m.readWatchEventCmd()

	case service.WatchEventSatisfied:
		m.satisfied = true
		m.confirmingExit = true
		return m, nil

	case service.WatchEventManualConfirm:
		m.satisfied = true
		m.confirmingExit = true
		return m, nil

	case service.WatchEventError:
		m.err = event.Error
		m.statusBar.SetError(event.Error)
		// Continue watching even after errors
		return m, m.readWatchEventCmd()

	case service.WatchEventPolling, service.WatchEventCooldown:
		// Update last checked time for polling events
		if event.Type == service.WatchEventPolling {
			m.statusBar.LastChecked = event.Timestamp
		}
		return m, m.readWatchEventCmd()
	}

	return m, m.readWatchEventCmd()
}

// scrollToBottom scrolls to show the latest content
func (m *Model) scrollToBottom() {
	// Calculate total lines from thoughts
	totalLines := len(m.thoughts)
	viewHeight := m.height - statusBarHeight - helpHeight - headerHeight
	if viewHeight < minViewportHeight {
		viewHeight = minViewportHeight
	}

	if totalLines > viewHeight {
		m.scrollOffset = totalLines - viewHeight
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

func (m *Model) startReviewCmd() tea.Cmd {
	m.fetching = true
	m.complete = false
	return func() tea.Msg {
		review, thoughts, err := m.reviewService.StartReview(m.ctx, m.config)
		if err != nil {
			return ErrorMsg{Err: err}
		}

		if thoughts == nil {
			// No comments to review - satisfied
			return ReviewCompleteMsg{Review: review}
		}

		return ReviewStartedMsg{Review: review, Thoughts: thoughts}
	}
}

func (m *Model) startWatchCmd() tea.Cmd {
	return func() tea.Msg {
		m.watchChan = m.watcher.Start(m.ctx, m.config.PRNumber)

		// Read first event
		select {
		case event, ok := <-m.watchChan:
			if !ok {
				return nil
			}
			return WatchEventMsg{Event: event}
		case <-m.ctx.Done():
			return nil
		}
	}
}

func (m *Model) readThoughtCmd() tea.Cmd {
	return func() tea.Msg {
		if m.thoughtsChan == nil {
			return nil
		}

		select {
		case thought, ok := <-m.thoughtsChan:
			if !ok {
				// Channel closed - review complete
				return ReviewCompleteMsg{Review: m.review}
			}
			return ThoughtMsg{Thought: thought}
		case <-m.ctx.Done():
			return nil
		}
	}
}

func (m *Model) readWatchEventCmd() tea.Cmd {
	return func() tea.Msg {
		if m.watchChan == nil {
			return nil
		}

		select {
		case event, ok := <-m.watchChan:
			if !ok {
				return nil
			}
			return WatchEventMsg{Event: event}
		case <-m.ctx.Done():
			return nil
		}
	}
}

func (m *Model) openPRCmd() tea.Cmd {
	return func() tea.Msg {
		// Use gh pr view --web to open in browser
		cmd := exec.Command("gh", "pr", "view", fmt.Sprintf("%d", m.config.PRNumber), "--web")
		_ = cmd.Run() // Ignore errors - best effort
		return nil
	}
}

// GetReview returns the current review
func (m *Model) GetReview() *domain.Review {
	return m.review
}

// IsComplete returns true if the review is complete
func (m *Model) IsComplete() bool {
	if m.review == nil {
		return false
	}
	return m.review.Status == domain.ReviewStatusCompleted ||
		m.review.Status == domain.ReviewStatusSatisfied
}

// buildCommentSummary creates initial thoughts showing the comments being addressed
func (m *Model) buildCommentSummary(review *domain.Review) []domain.ThoughtChunk {
	if review == nil || len(review.Comments) == 0 {
		return []domain.ThoughtChunk{}
	}

	now := time.Now()
	thoughts := []domain.ThoughtChunk{
		{
			Timestamp: now,
			Content:   fmt.Sprintf("─── CodeRabbit Comments (%d) ───", len(review.Comments)),
			Type:      domain.ThoughtTypeHeader,
		},
	}

	for i, comment := range review.Comments {
		// Build location string
		location := comment.Location()

		// Format: [1] path/to/file.go:42
		header := fmt.Sprintf("[%d] %s", i+1, location)
		thoughts = append(thoughts, domain.ThoughtChunk{
			Timestamp: now,
			Content:   header,
			Type:      domain.ThoughtTypeComment,
			File:      comment.FilePath,
		})

		// Show full comment body (word wrapped by renderer)
		body := comment.EffectiveBody()
		if body != "" {
			thoughts = append(thoughts, domain.ThoughtChunk{
				Timestamp: now,
				Content:   body,
				Type:      domain.ThoughtTypeComment,
				File:      comment.FilePath,
			})
		}
	}

	// Add separator before Claude's response
	thoughts = append(thoughts, domain.ThoughtChunk{
		Timestamp: now,
		Content:   "─── Claude's Analysis ───",
		Type:      domain.ThoughtTypeHeader,
	})

	return thoughts
}

// findNewline returns the index of the first newline, or -1
func findNewline(s string) int {
	for i, c := range s {
		if c == '\n' || c == '\r' {
			return i
		}
	}
	return -1
}
