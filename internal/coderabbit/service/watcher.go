package service

import (
	"context"
	"sync"
	"time"

	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
)

// WatchEventType represents the type of watch event
type WatchEventType string

const (
	WatchEventNewComments    WatchEventType = "new_comments"
	WatchEventNewCIFailures  WatchEventType = "new_ci_failures"
	WatchEventReviewComplete WatchEventType = "review_complete"
	WatchEventSatisfied      WatchEventType = "satisfied"
	WatchEventError          WatchEventType = "error"
	WatchEventCooldown       WatchEventType = "cooldown"
	WatchEventPolling        WatchEventType = "polling"
	WatchEventManualConfirm  WatchEventType = "manual_confirm"
)

// WatchEvent represents an event in watch mode
type WatchEvent struct {
	Type        WatchEventType
	Review      *domain.Review
	Thoughts    <-chan domain.ThoughtChunk
	Error       error
	Timestamp   time.Time
	Message     string
	Satisfied   SatisfactionResult
}

// WatchState represents the current state of the watcher
type WatchState string

const (
	WatchStateIdle       WatchState = "idle"
	WatchStatePolling    WatchState = "polling"
	WatchStateBatchWait  WatchState = "batch_wait"
	WatchStateProcessing WatchState = "processing"
	WatchStateCooldown   WatchState = "cooldown"
	WatchStateSatisfied  WatchState = "satisfied"
	WatchStateError      WatchState = "error"
)

// Watcher monitors a PR for changes and triggers reviews
type Watcher struct {
	service            *ReviewService
	detector           *SatisfactionDetector
	opts               WatchOptions
	mu                 sync.Mutex
	state              WatchState
	lastCommitSHA      string
	lastCommentCount   int
	lastCIFailureCount int  // Track CI failures to detect new ones
	processedCIOnce    bool // Have we processed CI failures for this commit?
	cooldownUntil      time.Time
	batchWaitUntil     time.Time
	review             *domain.Review
}

// NewWatcher creates a new watcher
func NewWatcher(service *ReviewService, opts WatchOptions) *Watcher {
	return &Watcher{
		service:  service,
		detector: NewSatisfactionDetector(),
		opts:     opts,
		state:    WatchStateIdle,
	}
}

// Start begins watching for changes and returns a channel of events
func (w *Watcher) Start(ctx context.Context, prNumber int) <-chan WatchEvent {
	events := make(chan WatchEvent, 10)

	go func() {
		defer close(events)

		ticker := time.NewTicker(w.opts.PollInterval)
		defer ticker.Stop()

		// Initial check
		w.checkForChanges(ctx, prNumber, events)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.checkForChanges(ctx, prNumber, events)
			}
		}
	}()

	return events
}

// checkForChanges polls for new comments or CI failures
func (w *Watcher) checkForChanges(ctx context.Context, prNumber int, events chan<- WatchEvent) {
	// Check current state (thread-safe read)
	w.mu.Lock()
	currentState := w.state
	cooldownUntil := w.cooldownUntil
	w.mu.Unlock()

	// Skip if we're already processing a review
	if currentState == WatchStateProcessing {
		events <- WatchEvent{
			Type:      WatchEventPolling,
			Timestamp: time.Now(),
			Message:   "Review in progress, waiting...",
		}
		return
	}

	// Check if we're in cooldown
	if currentState == WatchStateCooldown && time.Now().Before(cooldownUntil) {
		events <- WatchEvent{
			Type:      WatchEventCooldown,
			Timestamp: time.Now(),
			Message:   "In cooldown period",
		}
		return
	}

	// Exit cooldown if expired (thread-safe write)
	if currentState == WatchStateCooldown && time.Now().After(cooldownUntil) {
		w.mu.Lock()
		w.state = WatchStatePolling
		w.mu.Unlock()
	}

	// Signal polling
	events <- WatchEvent{
		Type:      WatchEventPolling,
		Timestamp: time.Now(),
		Message:   "Checking for new comments...",
	}

	// Fetch current review data
	config := ReviewConfig{
		PRNumber:        prNumber,
		IncludeNits:     w.opts.IncludeNits,
		IncludeOutdated: w.opts.IncludeOutdated,
	}

	review, err := w.service.FetchReviewData(ctx, config)
	if err != nil {
		events <- WatchEvent{
			Type:      WatchEventError,
			Error:     err,
			Timestamp: time.Now(),
			Message:   "Failed to fetch review data",
		}
		return
	}

	// Check for new comments
	newComments := len(review.Comments) > w.lastCommentCount
	newCommit := review.HeadCommit != w.lastCommitSHA

	// Check if satisfied (no actionable items AND all CI complete AND CodeRabbit has reviewed)
	codeRabbitReviewed := review.CodeRabbitFound && review.CodeRabbitCompleted
	if len(review.Comments) == 0 && len(review.CIFailures) == 0 && review.CIAllComplete && codeRabbitReviewed {
		// Check CodeRabbit's actual review status
		satisfaction, _ := w.service.CheckSatisfaction(ctx, review)

		if satisfaction.IsSatisfied {
			if w.opts.RequireManualConfirm {
				events <- WatchEvent{
					Type:      WatchEventManualConfirm,
					Review:    review,
					Timestamp: time.Now(),
					Message:   "Review appears satisfied. Confirm to exit watch mode.",
					Satisfied: satisfaction,
				}
			} else {
				w.mu.Lock()
				w.state = WatchStateSatisfied
				w.mu.Unlock()
				events <- WatchEvent{
					Type:      WatchEventSatisfied,
					Review:    review,
					Timestamp: time.Now(),
					Message:   "CodeRabbit is satisfied!",
					Satisfied: satisfaction,
				}
			}
			return
		}
	}

	// Check for new CI failures
	newCIFailures := len(review.CIFailures) > w.lastCIFailureCount

	// Reset CI processing flag on new commit
	if newCommit {
		w.processedCIOnce = false
	}

	// Update tracking state
	w.lastCommitSHA = review.HeadCommit
	w.lastCommentCount = len(review.Comments)
	w.lastCIFailureCount = len(review.CIFailures)

	// Determine if we need to process
	needsProcessing := false
	var eventType WatchEventType

	if newComments {
		needsProcessing = true
		eventType = WatchEventNewComments
	} else if len(review.Comments) > 0 {
		// Existing comments that need addressing
		needsProcessing = true
		eventType = WatchEventNewComments
	} else if len(review.CIFailures) > 0 && (!w.processedCIOnce || newCIFailures) {
		// CI failures: process if we haven't yet for this commit, or if there are new failures
		needsProcessing = true
		eventType = WatchEventNewCIFailures
		w.processedCIOnce = true
	}

	if !needsProcessing {
		// Nothing to do - send a polling event so UI knows we're still checking
		events <- WatchEvent{
			Type:      WatchEventPolling,
			Review:    review,
			Timestamp: time.Now(),
			Message:   "Checking for updates...",
		}
		return
	}

	// Batch wait - let more comments roll in before processing
	if w.opts.BatchWaitDuration > 0 {
		w.mu.Lock()
		w.state = WatchStateBatchWait
		w.batchWaitUntil = time.Now().Add(w.opts.BatchWaitDuration)
		w.mu.Unlock()

		events <- WatchEvent{
			Type:      WatchEventPolling,
			Review:    review,
			Timestamp: time.Now(),
			Message:   "Waiting for more comments to arrive...",
		}

		// Wait for batch duration
		select {
		case <-ctx.Done():
			return
		case <-time.After(w.opts.BatchWaitDuration):
		}

		// Re-fetch to get any new comments that came in during batch wait
		review, err = w.service.FetchReviewData(ctx, config)
		if err != nil {
			events <- WatchEvent{
				Type:      WatchEventError,
				Error:     err,
				Timestamp: time.Now(),
				Message:   "Failed to fetch review data after batch wait",
			}
			return
		}

		// Update tracking with new count
		w.lastCommentCount = len(review.Comments)
	}

	// Start processing (thread-safe)
	w.mu.Lock()
	w.state = WatchStateProcessing
	w.review = review
	w.mu.Unlock()

	// Start the actual review
	review, thoughts, err := w.service.StartReview(ctx, config)
	if err != nil {
		events <- WatchEvent{
			Type:      WatchEventError,
			Error:     err,
			Timestamp: time.Now(),
			Message:   "Failed to start review",
		}
		w.mu.Lock()
		w.state = WatchStatePolling
		w.mu.Unlock()
		return
	}

	// Emit event with thoughts channel
	events <- WatchEvent{
		Type:      eventType,
		Review:    review,
		Thoughts:  thoughts,
		Timestamp: time.Now(),
		Message:   "Processing new items...",
	}

	// Wait for review to complete in background
	go func() {
		// Handle nil thoughts channel (review already satisfied)
		if thoughts == nil {
			goto done
		}

		// Drain the thoughts channel (context-aware)
		for {
			select {
			case _, ok := <-thoughts:
				if !ok {
					// Channel closed - review complete
					goto done
				}
			case <-ctx.Done():
				// Context cancelled - exit goroutine
				return
			}
		}
	done:

		// Review complete
		events <- WatchEvent{
			Type:      WatchEventReviewComplete,
			Review:    review,
			Timestamp: time.Now(),
			Message:   "Review iteration complete",
		}

		// Enter cooldown (thread-safe)
		w.mu.Lock()
		w.state = WatchStateCooldown
		w.cooldownUntil = time.Now().Add(w.opts.CooldownDuration)
		w.mu.Unlock()

		events <- WatchEvent{
			Type:      WatchEventCooldown,
			Timestamp: time.Now(),
			Message:   "Entering cooldown period",
		}
	}()
}

// ConfirmSatisfied manually confirms that the review is satisfied
func (w *Watcher) ConfirmSatisfied() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.state = WatchStateSatisfied
}

// RejectSatisfied indicates to continue watching
func (w *Watcher) RejectSatisfied() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.state = WatchStatePolling
}

// GetState returns the current watcher state
func (w *Watcher) GetState() WatchState {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state
}

// GetCooldownRemaining returns the time remaining in cooldown
func (w *Watcher) GetCooldownRemaining() time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.state != WatchStateCooldown {
		return 0
	}
	remaining := time.Until(w.cooldownUntil)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetBatchWaitRemaining returns the time remaining in batch wait
func (w *Watcher) GetBatchWaitRemaining() time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.state != WatchStateBatchWait {
		return 0
	}
	remaining := time.Until(w.batchWaitUntil)
	if remaining < 0 {
		return 0
	}
	return remaining
}
