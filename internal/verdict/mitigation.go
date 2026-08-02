package verdict

// Mitigation is the concrete advice attached to a category.
type Mitigation struct {
	Summary string   `json:"summary"`
	Steps   []string `json:"steps"`
	// Owner hints at who should act, which is what actually gets a flake fixed.
	Owner string `json:"owner"`
}

// mitigations is a static lookup rather than something the model produces, so
// the advice a maintainer reads is reviewed and consistent. The model's own
// suggestion is kept alongside it as a secondary, per-case hint.
var mitigations = map[Category]Mitigation{
	NetworkTimeout: {
		Summary: "Make the network call tolerant of transient failure.",
		Steps: []string{
			"Wrap the call in a retry with exponential backoff and jitter.",
			"Raise the client timeout to cover realistic p99 latency, not the mean.",
			"Pin or vendor the dependency if the flake is a package registry fetch.",
			"Consider a local service double so the test does not depend on the public internet.",
		},
		Owner: "test author",
	},
	RaceCondition: {
		Summary: "Replace timing assumptions with explicit synchronisation.",
		Steps: []string{
			"Run the package with `go test -race` (or the language equivalent) to confirm.",
			"Replace sleeps with a channel, WaitGroup, or condition variable.",
			"Await the state you actually need rather than a fixed duration.",
			"Guard shared fixtures with a mutex, or give each test its own copy.",
		},
		Owner: "test author",
	},
	InfraFlake: {
		Summary: "Runner or platform problem — usually not the test's fault.",
		Steps: []string{
			"Re-run the job; confirm the failure does not reproduce on a fresh runner.",
			"Check the provider status page for the window in question.",
			"If it recurs on one runner label, pin the job to a different pool.",
			"Track the frequency — a rising rate is worth escalating to the platform team.",
		},
		Owner: "platform team",
	},
	ResourceExhaustion: {
		Summary: "The job ran out of memory, disk, or file descriptors.",
		Steps: []string{
			"Check peak usage against the runner's limits.",
			"Free disk before the step, or move to a larger runner.",
			"Reduce test parallelism so concurrent workers do not sum past the cap.",
			"Look for a leak: an unbounded cache or unclosed handles across the suite.",
		},
		Owner: "test author",
	},
	TestOrderDependency: {
		Summary: "The test depends on state left behind by another test.",
		Steps: []string{
			"Run the suite with shuffling enabled (`go test -shuffle=on`) to reproduce.",
			"Move shared setup into per-test fixtures with teardown.",
			"Avoid package-level mutable state shared between tests.",
			"Isolate anything global: temp dirs, env vars, registries, singletons.",
		},
		Owner: "test author",
	},
	GenuineBug: {
		Summary: "Not a flake — a real defect that reruns happen to mask.",
		Steps: []string{
			"Do not re-run to green. Open a bug with the failing assertion attached.",
			"Reproduce locally with the same inputs and seed.",
			"Check whether the failure rate correlates with a recent change.",
		},
		Owner: "code owner",
	},
	Unknown: {
		Summary: "Not enough signal in the log to classify with confidence.",
		Steps: []string{
			"Increase log verbosity for the failing step and wait for a recurrence.",
			"Upload test artifacts or a JUnit report so the next run has more to go on.",
			"Review manually — this occurrence needs a human.",
		},
		Owner: "maintainer",
	},
}

// MitigationFor returns the advice registered for c, falling back to the
// Unknown guidance for anything unrecognised.
func MitigationFor(c Category) Mitigation {
	if m, ok := mitigations[c]; ok {
		return m
	}
	return mitigations[Unknown]
}
