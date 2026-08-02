package llm

import (
	"fmt"
	"strings"
)

// SystemPrompt frames the model's role and, critically, forbids invented
// evidence.
//
// The citation rule is the load-bearing instruction. A verdict that quotes the
// log reads as authoritative, so a fabricated quote is worse than no quote at
// all — it manufactures false confidence. The instruction is reinforced by
// verdict.VerifyCitations, which drops any citation not literally present;
// prompting alone is not a guarantee, only a first line of defence.
const SystemPrompt = `You are a CI reliability engineer triaging a flaky test failure.

A flaky test is one that produced BOTH a pass and a failure on the SAME commit.
Your job is to read the failure log excerpt and decide the most likely root cause.

Rules you must follow:
1. Answer with a single JSON object and nothing else. No prose before or after.
2. "cited_lines" must contain ONLY lines copied verbatim from the excerpt you were
   given. Never invent, paraphrase, reformat, or reconstruct a line. If the excerpt
   contains no line that supports your verdict, return an empty array.
3. If the excerpt does not contain enough evidence, use the category "unknown" with a
   low confidence. An honest "unknown" is more useful than a confident guess.
4. "confidence" is your genuine probability that the category is correct, from 0 to 1.
   Do not default to 0.9.

Category definitions:
- network_timeout: a network call timed out, was refused, or DNS failed (dial tcp,
  i/o timeout, connection reset, TLS handshake timeout, 5xx from a registry).
- race_condition: concurrent access to shared state, or a test that depends on timing
  (race detector output, "send on closed channel", deadlock, sleep-based waits).
- infra_flake: the runner or platform failed, not the code (runner lost communication,
  shutdown signal, disk image error, spot reclamation, action download failure).
- resource_exhaustion: out of memory, out of disk, too many open files, OOMKilled,
  no space left on device.
- test_order_dependency: the test only fails in a particular order or with shuffling,
  or leaked state from another test (shared fixture, global registry, temp dir reuse).
- genuine_bug: a real assertion failure in application logic that reruns happen to
  mask. Nil dereference, wrong value, off-by-one.
- unknown: not enough evidence in the excerpt.`

// UserPromptTemplate is filled per occurrence.
const userPromptTemplate = `Job: %s
Failing step: %s

Failure log excerpt (this is the ONLY text you may cite from):
---BEGIN EXCERPT---
%s
---END EXCERPT---

Respond with exactly this JSON shape:
%s`

// BuildPrompt renders the full user prompt for a request.
func BuildPrompt(req Request) string {
	job := strings.TrimSpace(req.JobName)
	if job == "" {
		job = "(unknown job)"
	}
	step := strings.TrimSpace(req.StepName)
	if step == "" {
		step = "(unknown step)"
	}

	return fmt.Sprintf(userPromptTemplate, job, step, req.Excerpt, SchemaJSON)
}

// RepairPrompt is appended on the single retry after a malformed response. It
// restates only the format requirement, since the analysis itself may have been
// fine and only the envelope was wrong.
const RepairPrompt = `Your previous response could not be parsed.
Respond again with ONE valid JSON object and no other text.
"category" must be exactly one of: network_timeout, race_condition, infra_flake,
resource_exhaustion, test_order_dependency, genuine_bug, unknown.`
