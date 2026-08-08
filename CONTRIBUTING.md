# Contributing to flakehunter

## Development environment

**Requirements**

- Go 1.22 or newer
- `golangci-lint` for the lint target
- [Ollama](https://ollama.com) if you want to exercise the real LLM path — everything else works without it

```bash
git clone https://github.com/mdryaaan/flakehunter.git
cd flakehunter
go mod download
make build
```

Verify your setup with the offline pipeline. It needs no token, no network and no model:

```bash
make demo             # detection against the bundled fixtures
make pipeline         # scan -> classify -> report, end to end
make eval-baseline    # reproduce the baseline accuracy table
make test
```

For the LLM path:

```bash
ollama serve
ollama pull llama3
make eval
```

| Target | What it does |
| --- | --- |
| `make build` | Build into `bin/flakehunter` with version metadata |
| `make test` | Run all tests with coverage |
| `make cover` | Coverage profile plus a total |
| `make eval` | Accuracy harness against local Ollama |
| `make eval-baseline` | Accuracy harness against the rule-based baseline |
| `make lint` | golangci-lint |
| `make fmt` | gofmt |
| `make demo` | Offline detection demo |
| `make pipeline` | Full offline pipeline |

---

## Adding a verdict category

Categories are a closed set — the schema validator rejects anything outside it — so adding one touches four places:

1. **`internal/verdict/category.go`** — add the constant, add it to `AllCategories()`, give it a `Label()` and a `Severity()`. Severity drives report ordering, so decide honestly how urgent it is relative to `genuine_bug`.
2. **`internal/verdict/mitigation.go`** — add a `Mitigation` with a summary, concrete steps, and an owner. `TestMitigationForCoversEveryCategory` fails until you do.
3. **`internal/llm/prompt.go`** — add the category to the definition list in `SystemPrompt`. Describe the *evidence* that distinguishes it, not just the name; the model has nothing else to go on.
4. **`testdata/eval/labeled-cases.json`** — add at least five labelled cases, or the new category has no measured accuracy and you have no idea whether the model can recognise it.

Optionally add rules to `internal/llm/deterministic.go` so the baseline can compete on the new category too.

Run `make eval-baseline` before and after. If overall accuracy drops, the new category is being confused with an existing one and the prompt needs to distinguish them more sharply.

---

## Extending the eval corpus

The corpus is the project's most valuable asset — it is what turns prompt changes from guesswork into engineering.

Each case is one entry in `testdata/eval/labeled-cases.json` plus a log file in `testdata/eval/eval-logs/`:

```json
{
  "id": "case-041",
  "log": "eval-logs/case-041.log",
  "expected_category": "resource_exhaustion",
  "note": "Why a maintainer would label it this way"
}
```

Guidelines that matter:

- **Use real log shapes.** Copy the phrasing real tools emit. A corpus of invented text measures nothing useful.
- **Label the cause a maintainer would act on**, not every cause present. A test that OOMs *because* of a leak is `resource_exhaustion` — that is what you would fix first.
- **Label genuinely ambiguous excerpts `unknown` on purpose.** A classifier that guesses confidently on those is worse than one that abstains, and without such cases you cannot measure the difference.
- **Keep the distribution roughly even.** Five to six cases per category. A lopsided corpus makes accuracy a measure of the largest class.
- **Never tune a label to make a score look better.** If the model disagrees and the model is right, fix the label and say so in the PR. If it is wrong, leave the label alone.

`LoadCorpus` rejects an unknown category, so a typo in ground truth fails loudly rather than silently capping your measured accuracy.

---

## Adding an LLM provider

Implement the `llm.Provider` interface — `Name()`, `Model()`, `Classify()` — and register it in `llm.New`. It is deliberately a three-method interface; everything the pipeline needs from a model is "read this log, return this schema".

Your provider must:

- Send `SystemPrompt` and `BuildPrompt(req)` unchanged, so results stay comparable across providers
- Parse responses through `ParseVerdict`, never with its own JSON handling
- Retry exactly once on `ErrMalformed`, appending `RepairPrompt`, and not retry transport errors — a 500 is not fixed by asking the model more firmly
- Have a test using `httptest` that covers the clean path, the repair path, and a transport failure

---

## Branches and commits

Branch from `main` as `<type>/<short-description>`:

| Prefix | For |
| --- | --- |
| `feat/` | New capability |
| `fix/` | Bug fix |
| `test/` | Tests only |
| `docs/` | Documentation |
| `chore/` | Dependencies, tooling, CI |
| `refactor/` | Restructuring with no behaviour change |

Commits follow [Conventional Commits](https://www.conventionalcommits.org/), lowercase, imperative:

```
feat(detector): treat timed_out as a failing conclusion
fix(extractor): anchor error block on go test failure markers
test(llm): cover the malformed-response repair path
```

---

## Pull request checklist

- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `gofmt` clean (`make fmt`)
- [ ] `make eval-baseline` run, and the accuracy reported in the PR if it changed
- [ ] New exported functions have godoc comments
- [ ] Errors are wrapped with context (`fmt.Errorf("doing x: %w", err)`)
- [ ] No `panic` in library code
- [ ] New tests are table-driven and use `testify`
- [ ] `make demo` and `make pipeline` still work offline

If you changed the prompt, the schema, or a category, **include before-and-after eval numbers in the PR description.** That is the whole reason the harness exists — a prompt change without a measurement is a guess.

---

## Code of conduct

Be decent. Critique the code, not the person, and assume good faith.
