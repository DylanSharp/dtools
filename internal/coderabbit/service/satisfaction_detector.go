package service

import (
	"regexp"
	"strings"

	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
)

// SatisfactionDetector analyzes review content for satisfaction signals
type SatisfactionDetector struct {
	// Patterns that indicate satisfaction (review complete, no more issues)
	satisfactionPatterns []*regexp.Regexp

	// Patterns that indicate action is still required
	actionRequiredPatterns []*regexp.Regexp

	// Explicit satisfaction keywords
	satisfactionKeywords []string

	// Keywords that indicate issues remain
	issueKeywords []string
}

// NewSatisfactionDetector creates a new satisfaction detector
func NewSatisfactionDetector() *SatisfactionDetector {
	return &SatisfactionDetector{
		satisfactionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)looks?\s+good`),
			regexp.MustCompile(`(?i)LGTM`),
			regexp.MustCompile(`(?i)approved?`),
			regexp.MustCompile(`(?i)ready\s+to\s+merge`),
			regexp.MustCompile(`(?i)no\s+(further\s+)?issues?`),
			regexp.MustCompile(`(?i)no\s+(more\s+)?comments?`),
			regexp.MustCompile(`(?i)all\s+addressed`),
			regexp.MustCompile(`(?i)nothing\s+(else\s+)?to\s+(add|review)`),
			regexp.MustCompile(`(?i)changes?\s+look\s+good`),
			regexp.MustCompile(`(?i)✅.*addressed`),
			regexp.MustCompile(`(?i)✅.*fixed`),
		},
		actionRequiredPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)needs?\s+(to\s+)?(be\s+)?(change|fix|update|address|review)`),
			regexp.MustCompile(`(?i)should\s+(be\s+)?(change|fix|update|address)`),
			regexp.MustCompile(`(?i)must\s+(be\s+)?(change|fix|update|address)`),
			regexp.MustCompile(`(?i)requires?\s+(change|fix|update|attention)`),
			regexp.MustCompile(`(?i)please\s+(change|fix|update|address|review)`),
			regexp.MustCompile(`(?i)still\s+(has|have|need)`),
			regexp.MustCompile(`(?i)not\s+(yet\s+)?(address|fix|resolved)`),
			regexp.MustCompile(`(?i)issue\s+remain`),
			regexp.MustCompile(`(?i)bug\s+(in|found|detected)`),
		},
		satisfactionKeywords: []string{
			"SATISFIED",
			"COMPLETE",
			"DONE",
			"APPROVED",
			"SHIP IT",
		},
		issueKeywords: []string{
			"TODO",
			"FIXME",
			"BUG",
			"ERROR",
			"FAIL",
			"ISSUE",
			"PROBLEM",
		},
	}
}

// AnalyzeReview examines a review and determines if CodeRabbit is satisfied
func (d *SatisfactionDetector) AnalyzeReview(review *domain.Review) SatisfactionResult {
	result := SatisfactionResult{
		IsSatisfied:      false,
		Confidence:       0.0,
		Reasons:          []string{},
		ActionRequired:   []string{},
	}

	// Analyze the latest thoughts
	recentThoughts := d.getRecentThoughts(review.Thoughts, 20)
	thoughtsText := d.combineThoughts(recentThoughts)

	// Check for explicit satisfaction signals
	satisfactionScore := 0
	actionScore := 0

	// Check satisfaction patterns
	for _, pattern := range d.satisfactionPatterns {
		if pattern.MatchString(thoughtsText) {
			satisfactionScore++
			result.Reasons = append(result.Reasons, "Found satisfaction pattern: "+pattern.String())
		}
	}

	// Check satisfaction keywords
	upper := strings.ToUpper(thoughtsText)
	for _, keyword := range d.satisfactionKeywords {
		if strings.Contains(upper, keyword) {
			satisfactionScore += 2 // Keywords are stronger signals
			result.Reasons = append(result.Reasons, "Found satisfaction keyword: "+keyword)
		}
	}

	// Check action required patterns
	for _, pattern := range d.actionRequiredPatterns {
		if pattern.MatchString(thoughtsText) {
			actionScore++
			result.ActionRequired = append(result.ActionRequired, "Found action pattern: "+pattern.String())
		}
	}

	// Check issue keywords
	for _, keyword := range d.issueKeywords {
		if strings.Contains(upper, keyword) {
			actionScore++
			result.ActionRequired = append(result.ActionRequired, "Found issue keyword: "+keyword)
		}
	}

	// Calculate confidence and determination
	totalSignals := satisfactionScore + actionScore
	if totalSignals == 0 {
		result.Confidence = 0.5 // No signals either way
	} else {
		result.Confidence = float64(satisfactionScore) / float64(totalSignals)
	}

	// Satisfaction requires:
	// 1. At least 2 satisfaction signals
	// 2. Satisfaction score > action score
	// 3. Confidence > 0.6
	result.IsSatisfied = satisfactionScore >= 2 &&
		satisfactionScore > actionScore &&
		result.Confidence > 0.6

	return result
}

// AnalyzeCodeRabbitReview examines the actual CodeRabbit review text
func (d *SatisfactionDetector) AnalyzeCodeRabbitReview(comments []domain.Comment) SatisfactionResult {
	result := SatisfactionResult{
		IsSatisfied:    false,
		Confidence:     0.0,
		Reasons:        []string{},
		ActionRequired: []string{},
	}

	// If there are no comments, we might be satisfied
	if len(comments) == 0 {
		result.IsSatisfied = true
		result.Confidence = 1.0
		result.Reasons = append(result.Reasons, "No CodeRabbit comments remaining")
		return result
	}

	// Check each comment for resolved status
	resolvedCount := 0
	unresolvedCount := 0

	for _, comment := range comments {
		if comment.IsResolved {
			resolvedCount++
		} else {
			unresolvedCount++
			result.ActionRequired = append(result.ActionRequired,
				"Unresolved comment on "+comment.Location())
		}
	}

	totalComments := resolvedCount + unresolvedCount
	if totalComments == 0 {
		result.IsSatisfied = true
		result.Confidence = 1.0
		return result
	}

	result.Confidence = float64(resolvedCount) / float64(totalComments)
	result.IsSatisfied = unresolvedCount == 0

	if result.IsSatisfied {
		result.Reasons = append(result.Reasons,
			"All CodeRabbit comments resolved")
	}

	return result
}

// getRecentThoughts returns the N most recent thoughts
func (d *SatisfactionDetector) getRecentThoughts(thoughts []domain.ThoughtChunk, n int) []domain.ThoughtChunk {
	if len(thoughts) <= n {
		return thoughts
	}
	return thoughts[len(thoughts)-n:]
}

// combineThoughts combines thought content into a single string
func (d *SatisfactionDetector) combineThoughts(thoughts []domain.ThoughtChunk) string {
	var parts []string
	for _, t := range thoughts {
		parts = append(parts, t.Content)
	}
	return strings.Join(parts, "\n")
}

// SatisfactionResult contains the results of satisfaction analysis
type SatisfactionResult struct {
	// IsSatisfied indicates if the review is satisfied
	IsSatisfied bool

	// Confidence is a value between 0 and 1 indicating confidence in the result
	Confidence float64

	// Reasons lists why we think it's satisfied
	Reasons []string

	// ActionRequired lists outstanding items that need attention
	ActionRequired []string
}
