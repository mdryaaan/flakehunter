<div align="center">

# 🔍 flakehunter

**Finds flaky tests in your GitHub Actions CI, uses an LLM to explain why they're flaky, and tells you how to fix them — runs fully offline with local models, no API key required.**

[![CI](https://github.com/mdryaaan/flakehunter/actions/workflows/ci.yml/badge.svg)](https://github.com/mdryaaan/flakehunter/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/mdryaaan/flakehunter?color=00ADD8&logo=github)](https://github.com/mdryaaan/flakehunter/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)
[![Ollama](https://img.shields.io/badge/LLM-Ollama%20by%20default-000000)](https://ollama.com)

</div>

---

## Features

- **Real flake detection, not "any red run"** — a job counts as flaky only when the *same job*, on the *same commit*, produced both a pass and a failure. A red commit followed by a green one is someone fixing the build.
- **LLM root-cause classification** into seven categories, with a confidence score and cited evidence
- **Schema-constrained output** — every verdict is validated against a fixed JSON contract; malformed responses get one repair attempt, then fail honestly
- **Fabricated citations are dropped** — every quoted line is checked against the excerpt it claims to come from
- **Measured accuracy** — a 40-case hand-labelled corpus, with precision, recall, F1 and a confusion matrix
- **Ollama by default** — no API key, no CI logs leaving your machine
- **Offline mode** — the whole pipeline runs against bundled fixtures with no token and no network
- **Context-aware chunking** — keeps the command echo and the failure window, not a blind byte slice
- **Four output formats** — markdown report, JSON, GitHub issue body, weekly digest
- **Ships as a GitHub Action** — drop-in weekly flake digest for your own repo

---

## Architecture

<p align="center">
  <img src="./docs/arch.png"
       alt="flakehunter pipeline: the GitHub Actions API feeds scan (internal/github + internal/detector), which yields flaky occurrences; internal/extractor pulls the failure logs and produces a chunked excerpt; internal/llm classifies it via Ollama, the Claude API, or the deterministic rule-based baseline; the structured verdict carries category, confidence and citations, and internal/report renders a markdown report, a GitHub issue body, or a weekly digest. A separate eval harness scores the classifier against 40 labelled fixtures."
       width="620" />
</p>

---

## Evaluation results

**An LLM classifier without a measured accuracy is a demo, not a tool.** You cannot tell whether a prompt change helped, whether a smaller model is good enough, or whether the model is beating a regex — so flakehunter ships an eval harness and a labelled corpus, and `make eval-baseline` reproduces the numbers below on any machine.

### Baseline — deterministic rule-based classifier

> [!IMPORTANT]
> **These are baseline numbers. No language model was involved.**
> They come from `--provider deterministic`, a transparent keyword-and-regex classifier. It exists as a *control*: "the model scored 82%" is meaningless alone, while "the model scored 82% where regex scores 77.5%" is a result. A classifier that cannot beat this floor is not earning its inference cost.

Reproduce with `make eval-baseline` — no Ollama, no API key, no network:

```
Overall
  cases           40
  correct         31
  accuracy        77.5%
  macro F1        0.781
  mean confidence 0.79 when right, 0.42 when wrong
  fabricated citations dropped: 0
```

| Category | Support | Predicted | Precision | Recall | F1 |
| --- | ---: | ---: | ---: | ---: | ---: |
| genuine_bug | 5 | 6 | 0.67 | 0.80 | 0.73 |
| infra_flake | 6 | 3 | 1.00 | 0.50 | 0.67 |
| network_timeout | 6 | 6 | 0.83 | 0.83 | 0.83 |
| race_condition | 6 | 5 | 1.00 | 0.83 | 0.91 |
| resource_exhaustion | 6 | 5 | 1.00 | 0.83 | 0.91 |
| test_order_dependency | 6 | 4 | 1.00 | 0.67 | 0.80 |
| unknown | 5 | 11 | 0.45 | 1.00 | 0.62 |

```
actual \ predicted     net  race infra   res order   bug   unk
--------------------------------------------------------------
net                      5     .     .     .     .     .     1
race                     .     5     .     .     .     1     .
infra                    .     .     3     .     .     .     3
res                      1     .     .     5     .     .     .
order                    .     .     .     .     4     1     1
bug                      .     .     .     .     .     4     1
unk                      .     .     .     .     .     .     5
```

### What the numbers actually say

Three things are worth reading off that matrix, and they are the reason the harness exists:

**The baseline's weakness is recall on `infra_flake` (0.50).** Three of six infra cases fall through to `unknown` — cache-service 503s and spot reclamations are phrased too many ways for a fixed pattern list. This is precisely the gap a language model should close, and precisely what the harness will measure when you run it against one.

**`unknown` has 1.00 recall but 0.45 precision.** The baseline abstains rather than guessing, which is the behaviour the confidence floor is designed to produce. Over-predicting `unknown` is the *safe* failure mode: a maintainer can act on "I don't know", and is actively misled by a confident wrong label.

**Confidence is calibrated.** Mean confidence is 0.79 when the verdict is right and 0.42 when it is wrong. The score carries real signal, which is what makes `--min-confidence` a meaningful control rather than a placebo.

### LLM results

Not measured here — this machine has no Ollama instance and no API key, and **I will not publish numbers a model did not produce.** To generate them:

```bash
ollama serve && ollama pull llama3
make eval                       # or: flakehunter eval --provider ollama --model llama3 --verbose
```

The harness prints the same table for any provider, so the comparison against the 77.5% baseline is direct.

---

## Why Ollama by default

Three reasons, in order of how much they matter:

1. **CI logs are not safe to ship to a third party.** They routinely carry internal hostnames, IPs, dependency inventories, and occasionally a secret that leaked into an error message. Local inference means the log never leaves the machine.
2. **A tool that needs a key does not get evaluated.** Anyone can clone this and run the full pipeline in under a minute. That is the difference between a project someone tries and one they read about.
3. **Classification is cheap work.** Deciding whether a log says "timeout" or "data race" does not need a frontier model, so paying per token for it is hard to justify at CI volume.

`--provider claude` is there for when accuracy matters more than those three things.

---

## Installation

**Prebuilt binary** — no Go toolchain needed. Linux, macOS and Windows on amd64 and arm64, with checksums and an SBOM on every [release](https://github.com/mdryaaan/flakehunter/releases/latest):

```bash
VERSION=0.1.0
curl -sSfL "https://github.com/mdryaaan/flakehunter/releases/download/v${VERSION}/flakehunter_${VERSION}_linux_amd64.tar.gz" \
  | tar -xz flakehunter
sudo install -m 0755 flakehunter /usr/local/bin/
flakehunter version
```

Each archive also ships `testdata/`, so the eval numbers below are reproducible straight from a release without cloning.

**With Go:**

```bash
go install github.com/mdryaaan/flakehunter@latest
```

**From source:**

```bash
git clone https://github.com/mdryaaan/flakehunter.git
cd flakehunter
make build          # -> bin/flakehunter
```

## Quick start — no credentials needed

```bash
# Detect flaky occurrences in the bundled fixtures
make demo

# Run the whole pipeline offline and print a markdown report
make pipeline

# Reproduce the baseline accuracy numbers above
make eval-baseline
```

Against a real repository:

```bash
export GITHUB_TOKEN=ghp_...          # optional for public repos, but raises the rate limit

flakehunter scan     --repo acme/orders-api --days 14 --output scan.json
flakehunter classify --input scan.json --provider ollama --model llama3 --output classified.json
flakehunter report   --input classified.json --format markdown
```

---

## Command reference

| Command | What it does |
| --- | --- |
| `flakehunter scan` | Fetch workflow runs and find jobs that both passed and failed on one commit |
| `flakehunter classify` | Download failure logs, chunk them, and get a structured verdict per occurrence |
| `flakehunter report` | Render results as `markdown`, `json`, `issue`, or `digest` |
| `flakehunter eval` | Score a provider against the labelled corpus |
| `flakehunter version` | Build metadata |
| `flakehunter completion` | Shell completion for bash, zsh, fish, powershell |

### Key flags

| Flag | Default | Notes |
| --- | --- | --- |
| `--provider` | `ollama` | `ollama`, `claude`, or `deterministic` |
| `--model` | per provider | e.g. `llama3`, `claude-sonnet-4-6` |
| `--min-confidence` | `0.5` | Verdicts below this are reported as `unknown` |
| `--temperature` | `0` | Classification wants repeatability, not creativity |
| `--offline` | `false` | Read from `--fixtures` instead of the API |
| `--days` | `7` | Scan window |
| `--workflow` | all | Restrict to one workflow file |

---

## Adopting the GitHub Action

Copy [`.github/workflows/flakehunter-action.yml`](.github/workflows/flakehunter-action.yml) into your own repository. It runs weekly, scans the last seven days, classifies every flaky occurrence, and opens an issue with the digest.

```yaml
name: Weekly flake digest
on:
  schedule: [{ cron: '0 6 * * 1' }]
  workflow_dispatch:

permissions:
  contents: read
  issues: write

jobs:
  digest:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: |
          curl -sSfL https://github.com/mdryaaan/flakehunter/releases/latest/download/flakehunter_0.1.0_linux_amd64.tar.gz \
            | tar -xz flakehunter
          sudo install -m 0755 flakehunter /usr/local/bin/
      - env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          flakehunter scan --repo "$GITHUB_REPOSITORY" --days 7 --output scan.json
          flakehunter classify --input scan.json --provider deterministic --output classified.json
          flakehunter report --input classified.json --format digest --since 7 --output digest.md
      - env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: gh issue create --title "Flake digest" --body-file digest.md
```

The shipped workflow downloads the prebuilt binary and only falls back to compiling from source if no matching release asset exists, so a scheduled run costs seconds rather than a Go build.

It deliberately **skips opening an issue when CI was clean** — a weekly "nothing to report" issue is exactly how a bot gets muted.

---

## Design notes

### The detection algorithm

The whole tool rests on one definition, and getting it wrong produces false positives that destroy trust faster than the flakes it finds:

> A job is flaky when the **same job name**, in the **same workflow**, on the **same commit SHA**, produced **both a pass and a failure**.

The commit SHA is the load-bearing part. Three cases that look similar and are not:

| Observation | Verdict | Why |
| --- | --- | --- |
| Job failed on `abc`, passed on `abc` (rerun) | **flaky** | The code did not change; the outcome did |
| Job failed on `abc`, passed on `def` | not flaky | Someone fixed the build |
| Job failed twice on `abc` | not flaky | Broken build |
| Job failed on `abc`, rerun cancelled | not flaky | A cancellation says nothing about the code |

This is why `scan` fetches *every attempt of every run* rather than the latest — reruns are where the entire signal lives. The bundled fixtures include all four of those cases as negative controls, and the CI smoke test asserts exactly four occurrences are found.

### Chunking strategy

A GitHub Actions log archive is a zip of per-step text files, often megabytes, against an LLM context measured in thousands of tokens. Two obvious approaches both fail:

- **Take the last N bytes** → you capture the runner's teardown and drop the stack trace.
- **Take the first N bytes** → you capture setup and never reach the failure.

flakehunter instead picks the failing step (skipping `Post *` cleanup steps, which mention "error" while unwinding and are a classic red herring), anchors on the last *specific* failure marker, and keeps a window around it plus the command echo from the top of the step. Elided material is replaced with an explicit marker so the model knows text was removed rather than inferring a gap in the story.

### Why structured output with citations

Three layers, because prompting alone is not a guarantee:

1. **A closed category set.** The model cannot invent a taxonomy; anything outside the seven categories is rejected by the schema validator.
2. **Verified citations.** The prompt forbids inventing log lines, and `VerifyCitations` then drops any cited line not literally present in the excerpt. A verdict that quotes the log reads as authoritative, so a *fabricated* quote is worse than no quote — it manufactures false confidence. The eval harness counts dropped citations as a first-class metric.
3. **A confidence floor.** Verdicts under `--min-confidence` are demoted to `unknown`, with the original label preserved for review. An honest "I don't know" is actionable; a confident wrong label is not.

---

## Project layout

```
cmd/                  cobra commands: scan, classify, report, eval, version, completion
internal/config/      flag resolution and validation
internal/github/      API client, pagination, rate-limit backoff, offline fixture source
internal/detector/    the flaky signature algorithm
internal/extractor/   zip archive reader, failing-step parser, context-aware chunker
internal/llm/         provider interface, Ollama, Claude, deterministic baseline, schema, prompt
internal/verdict/     categories, validation, citation verification, mitigation lookup
internal/report/      markdown, JSON, issue body, digest
internal/eval/        harness, scorer, confusion matrix
testdata/fixtures/    offline demo data — 4 flaky signatures + 4 negative controls
testdata/eval/        40 hand-labelled cases across all 7 categories
```

Test coverage: **64.6% overall**, with the logic-bearing packages at 83–100% (`config` 100%, `eval` 96%, `detector` 95%, `extractor` 94%, `utils` 94%, `llm` 90%, `report` 90%, `verdict` 83%). The uncovered remainder is CLI wiring and live-API paths.

---

## Roadmap

- [ ] Publish measured accuracy for `llama3`, `qwen2.5-coder`, and `claude-sonnet-4-6` against the same corpus
- [ ] Cross-run flake tracking, so a digest can report a rising or falling trend
- [ ] JUnit XML ingestion, for per-test rather than per-job granularity
- [ ] Automatic PR comment when a flake is detected on the PR's own branch
- [ ] Expand the corpus beyond 40 cases and beyond Go-flavoured logs
- [ ] Token and cost accounting per classification run

---

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) — dev setup, how to add a verdict category, and how to extend the eval corpus.

## License

[MIT](./LICENSE) © 2026 Md Raiyan
