package service

import (
	"fmt"
	"strings"

	"github.com/DylanSharp/dtools/internal/coderabbit/domain"
)

// PromptBuilder builds prompts for Claude from review data
type PromptBuilder struct{}

// NewPromptBuilder creates a new prompt builder
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

// BuildReviewPrompt generates a prompt for Claude to address CodeRabbit comments and CI failures
func (b *PromptBuilder) BuildReviewPrompt(review *domain.Review) string {
	var sections []string

	// Separate comments by type
	inlineComments := []domain.Comment{}
	outsideDiffComments := []domain.Comment{}
	nitpickComments := []domain.Comment{}

	for _, c := range review.Comments {
		if c.IsNit {
			nitpickComments = append(nitpickComments, c)
		} else if c.IsOutsideDiff {
			outsideDiffComments = append(outsideDiffComments, c)
		} else {
			inlineComments = append(inlineComments, c)
		}
	}

	// Process inline comments
	if len(inlineComments) > 0 {
		sections = append(sections, b.formatCommentSection("Inline Review Comments", inlineComments))
	}

	// Process outside diff comments
	if len(outsideDiffComments) > 0 {
		sections = append(sections, b.formatCommentSection("Outside Diff Range Comments", outsideDiffComments))
	}

	// Process nitpick comments
	if len(nitpickComments) > 0 {
		sections = append(sections, b.formatCommentSection("Nitpick Comments", nitpickComments))
	}

	// Process CI failures
	if len(review.CIFailures) > 0 {
		sections = append(sections, b.formatCIFailures(review.CIFailures))
	}

	// Build intro based on content
	hasFailures := len(review.CIFailures) > 0
	hasComments := len(review.Comments) > 0

	var intro string
	if hasFailures && hasComments {
		intro = `Please address the following CI/test failures AND review comments using extensible code and industry best practices.
Assess each comment to see if you agree with the comment. If you do, address the comment. If you do not, do not address the comment.
Each item is numbered.
Work through each item one by one. Keep track of your progress.`
	} else if hasFailures {
		intro = `Please fix the following CI/test failures using extensible code and industry best practices.`
	} else {
		intro = `Please address the following review comments using extensible code and industry best practices.
Assess each comment to see if you agree with the comment. If you do, address the comment. If you do not, do not address the comment.
Each item is numbered.
Work through each item one by one. Keep track of your progress.`
	}

	prompt := fmt.Sprintf(`%s

- Make minimal, safe edits aligned with project style.
- If a change requires design or product input, do NOT edit; instead, leave me a clear comment reply explaining the decision/tradeoffs.
- After making your changes, run the full suite of tests and linters and ensure they pass and there are no new errors or warnings.
- If you need more context on any one item you can use the github CLI tool (gh) to fetch more information from the pull request.
- If it's a python project:
	- Use black (locally installed) and autoflake to format the code.
	- Use flake8 (locally installed) to check for linting errors and fix them.
	- Run isort using docker-compose run --rm web python -m isort .
	- When you run tests with pytest, run them in parallel with -n auto.

When you are happy with the changes, commit the changes and push them to the branch.

%s`, intro, strings.Join(sections, "\n\n"))

	return prompt
}

// formatCommentSection formats a section of comments
func (b *PromptBuilder) formatCommentSection(title string, comments []domain.Comment) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("--- %s ---", title))

	// Group comments by file
	grouped := b.groupByFile(comments)

	commentNumber := 1
	for file, fileComments := range grouped {
		lines = append(lines, fmt.Sprintf("## %s", file))

		for _, comment := range fileComments {
			lineInfo := ""
			if comment.LineNumber > 0 {
				lineInfo = fmt.Sprintf("L%d", comment.LineNumber)
			}

			// Use AI prompt if available, otherwise full body
			body := comment.EffectiveBody()

			// Format as a numbered checkbox item
			lines = append(lines, fmt.Sprintf("- [ ] %d. %s (%s)", commentNumber, lineInfo, comment.URL))

			// Indent the body
			indentedBody := b.indentText(body, "   ")
			lines = append(lines, indentedBody)
			lines = append(lines, "")

			commentNumber++
		}
	}

	return strings.Join(lines, "\n")
}

// formatCIFailures formats CI test failures
func (b *PromptBuilder) formatCIFailures(failures []domain.CITestFailure) string {
	var lines []string
	lines = append(lines, "--- Failed CI Checks / Tests ---")
	lines = append(lines, "")

	for _, failure := range failures {
		lines = append(lines, fmt.Sprintf("## %s (%s)", failure.CheckName, failure.AppName))
		lines = append(lines, fmt.Sprintf("URL: %s", failure.LogURL))

		if failure.Summary != "" {
			lines = append(lines, fmt.Sprintf("Summary: %s", failure.Summary))
		}

		// Add annotations (specific failure locations)
		if len(failure.Annotations) > 0 {
			lines = append(lines, "")
			lines = append(lines, "Failure Details:")
			for _, annotation := range failure.Annotations {
				location := fmt.Sprintf("L%d", annotation.StartLine)
				if annotation.StartLine != annotation.EndLine {
					location = fmt.Sprintf("L%d-%d", annotation.StartLine, annotation.EndLine)
				}

				lines = append(lines, fmt.Sprintf("- %s:%s", annotation.Path, location))
				if annotation.Title != "" {
					lines = append(lines, fmt.Sprintf("  Title: %s", annotation.Title))
				}
				lines = append(lines, fmt.Sprintf("  %s", annotation.Message))
				if annotation.RawDetails != "" {
					indented := b.indentText(annotation.RawDetails, "    ")
					lines = append(lines, indented)
				}
			}
		}

		// Add full output if available and no annotations
		if failure.ErrorMessage != "" && len(failure.Annotations) == 0 {
			lines = append(lines, "")
			lines = append(lines, "Test Output:")
			lines = append(lines, "```")
			lines = append(lines, failure.ErrorMessage)
			lines = append(lines, "```")
		}

		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// groupByFile groups comments by their file path
func (b *PromptBuilder) groupByFile(comments []domain.Comment) map[string][]domain.Comment {
	grouped := make(map[string][]domain.Comment)

	for _, comment := range comments {
		file := comment.FilePath
		if file == "" {
			file = "GENERAL"
		}
		grouped[file] = append(grouped[file], comment)
	}

	return grouped
}

// indentText indents each line of text with the given prefix
func (b *PromptBuilder) indentText(text, indent string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}
