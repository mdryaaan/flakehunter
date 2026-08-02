package extractor

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func buildLog(head, filler, failure string, fillerLines int) string {
	var b strings.Builder
	b.WriteString(head + "\n")
	for i := 0; i < fillerLines; i++ {
		fmt.Fprintf(&b, "%s %d\n", filler, i)
	}
	b.WriteString(failure + "\n")
	b.WriteString("post-failure summary line\n")
	return b.String()
}

func TestChunkKeepsFailureAnchor(t *testing.T) {
	body := buildLog("$ go test ./...", "ok  package/noise", "--- FAIL: TestThing (0.02s)", 500)
	got := Chunk(StepLog{Name: "Run tests", Body: body}, DefaultChunkOptions())

	assert.Contains(t, got.Text, "--- FAIL: TestThing",
		"the failure anchor must survive chunking")
	assert.True(t, got.Truncated)
	assert.Less(t, got.ExcerptBytes, got.OriginalBytes)
	assert.LessOrEqual(t, got.ExcerptBytes, DefaultChunkOptions().MaxChars)
}

func TestChunkKeepsCommandEcho(t *testing.T) {
	body := buildLog("$ go test -race ./...", "noise", "panic: send on closed channel", 400)
	got := Chunk(StepLog{Name: "Run tests", Body: body}, DefaultChunkOptions())

	assert.Contains(t, got.Text, "go test -race",
		"the command echo explains what was actually run")
	assert.Contains(t, got.Text, "panic: send on closed channel")
	assert.Contains(t, got.Text, "truncated by flakehunter",
		"elision must be explicit so the model knows material was removed")
}

func TestChunkShortLogUntouched(t *testing.T) {
	body := "$ make test\nall good\n--- FAIL: TestA\nexpected 1 got 2\n"
	got := Chunk(StepLog{Name: "Run tests", Body: body}, DefaultChunkOptions())

	assert.Contains(t, got.Text, "--- FAIL: TestA")
	assert.Contains(t, got.Text, "expected 1 got 2")
	assert.False(t, got.Truncated)
}

func TestChunkRespectsBudget(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 20000; i++ {
		fmt.Fprintf(&b, "line of log output number %d\n", i)
	}
	b.WriteString("##[error]Process completed with exit code 1.\n")

	opts := ChunkOptions{MaxChars: 2000, LinesBefore: 5000, LinesAfter: 5000, HeadLines: 5}
	got := Chunk(StepLog{Name: "Run tests", Body: b.String()}, opts)

	assert.LessOrEqual(t, got.ExcerptBytes, opts.MaxChars+80,
		"excerpt must respect the character budget")
	assert.True(t, got.Truncated)
}

func TestErrorBlockAnchorsOnLastSpecificMarker(t *testing.T) {
	body := strings.Join([]string{
		"starting",
		"error: a misleading early mention",
		"lots of progress",
		"--- FAIL: TestReal (1.20s)",
		"    thing_test.go:44: boom",
		"FAIL",
	}, "\n")

	got := ErrorBlock(body, 2, 2)
	assert.Contains(t, got, "--- FAIL: TestReal")
	assert.Contains(t, got, "thing_test.go:44: boom")
}

func TestErrorBlockWithoutMarkerReturnsWholeBody(t *testing.T) {
	body := "nothing\ninteresting\nhere"
	assert.Equal(t, body, ErrorBlock(body, 3, 3))
}

func TestChunkZeroOptionsFallsBackToDefaults(t *testing.T) {
	got := Chunk(StepLog{Name: "s", Body: "--- FAIL: X\n"}, ChunkOptions{})
	assert.NotEmpty(t, got.Text)
}
