package extractor

import (
	"fmt"
	"strings"

	"github.com/mdryaaan/flakehunter/internal/utils"
)

// ChunkOptions tunes how a step log is reduced to an LLM-sized excerpt.
type ChunkOptions struct {
	// MaxChars is the hard ceiling on the excerpt. Characters rather than tokens
	// because the budget must hold for any provider; ~4 chars/token is the usual
	// rule of thumb, so 12000 chars is roughly 3k tokens.
	MaxChars int
	// LinesBefore and LinesAfter frame the failure anchor.
	LinesBefore int
	LinesAfter  int
	// HeadLines keeps the start of the step, where the command and environment
	// are echoed — often the difference between a confident and an unknown verdict.
	HeadLines int
}

// DefaultChunkOptions is a budget that fits comfortably in a small local model's
// context while still carrying a full stack trace.
func DefaultChunkOptions() ChunkOptions {
	return ChunkOptions{MaxChars: 12000, LinesBefore: 60, LinesAfter: 25, HeadLines: 12}
}

// Excerpt is the reduced log handed to a provider, plus what it cost.
type Excerpt struct {
	Text          string `json:"text"`
	StepName      string `json:"step_name"`
	OriginalBytes int    `json:"original_bytes"`
	ExcerptBytes  int    `json:"excerpt_bytes"`
	Truncated     bool   `json:"truncated"`
}

// Chunk reduces a step log to an excerpt that fits the budget.
//
// The strategy is deliberately not "take the last N bytes". A blind tail
// captures the runner's teardown and drops the stack trace, and a blind head
// captures setup and never reaches the failure. Instead the excerpt is built
// from the two regions that carry signal — the command echo at the top, and the
// window around the failure — joined by an explicit elision marker so the model
// knows material was removed rather than inferring a gap in the story.
func Chunk(step StepLog, opts ChunkOptions) Excerpt {
	if opts.MaxChars <= 0 {
		opts = DefaultChunkOptions()
	}

	body := utils.CollapseBlankLines(step.Body)
	original := len(step.Body)

	block := ErrorBlock(body, opts.LinesBefore, opts.LinesAfter)

	var builder strings.Builder
	head := headLines(body, opts.HeadLines)

	// Only prepend the head when it is genuinely separate material, otherwise
	// the excerpt repeats itself and wastes budget.
	if head != "" && !strings.Contains(block, head) {
		builder.WriteString(head)
		builder.WriteString("\n\n... [log truncated by flakehunter] ...\n\n")
	}
	builder.WriteString(block)

	text := builder.String()
	truncated := len(text) < len(body)

	if len(text) > opts.MaxChars {
		// Keep the tail of the excerpt: the failure anchor sits near its end.
		runes := []rune(text)
		if len(runes) > opts.MaxChars {
			runes = runes[len(runes)-opts.MaxChars:]
			text = "... [head of excerpt dropped to fit budget] ...\n" + string(runes)
		}
		truncated = true
	}

	return Excerpt{
		Text:          text,
		StepName:      step.Name,
		OriginalBytes: original,
		ExcerptBytes:  len(text),
		Truncated:     truncated,
	}
}

func headLines(body string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(body, "\n")
	if len(lines) <= n {
		return ""
	}
	return strings.Join(lines[:n], "\n")
}

// ChunkArchive is the whole path from raw archive bytes to an excerpt.
func ChunkArchive(data []byte, opts ChunkOptions) (Excerpt, error) {
	logs, err := ReadArchive(data)
	if err != nil {
		return Excerpt{}, fmt.Errorf("reading archive: %w", err)
	}

	step, ok := FailingStep(logs)
	if !ok {
		// No explicit failure marker anywhere. Fall back to the last step rather
		// than giving up: an exit code with no message is still worth showing.
		step = logs[len(logs)-1]
	}

	return Chunk(step, opts), nil
}
