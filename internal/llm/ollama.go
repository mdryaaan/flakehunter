package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mdryaaan/flakehunter/internal/verdict"
)

// DefaultOllamaURL is where Ollama listens out of the box.
const DefaultOllamaURL = "http://localhost:11434"

// DefaultOllamaModel is a small instruct model that fits on a laptop.
const DefaultOllamaModel = "llama3"

// Ollama talks to a local Ollama daemon.
//
// This is flakehunter's default provider on purpose: it needs no API key, sends
// no CI logs to a third party, and lets anyone evaluating the tool run the full
// pipeline immediately. CI logs routinely contain internal hostnames and
// occasionally leaked secrets, which is a real argument for local inference
// beyond convenience.
type Ollama struct {
	baseURL     string
	model       string
	temperature float64
	client      *http.Client
}

// NewOllama builds an Ollama provider, filling in defaults.
func NewOllama(opts Options) *Ollama {
	base := opts.BaseURL
	if base == "" {
		base = DefaultOllamaURL
	}
	model := opts.Model
	if model == "" {
		model = DefaultOllamaModel
	}

	return &Ollama{
		baseURL:     base,
		model:       model,
		temperature: opts.Temperature,
		client:      &http.Client{Timeout: 180 * time.Second},
	}
}

// Name identifies the provider.
func (o *Ollama) Name() string { return ProviderOllama }

// Model identifies the model in use.
func (o *Ollama) Model() string { return o.model }

type ollamaRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	System  string         `json:"system"`
	Stream  bool           `json:"stream"`
	Format  string         `json:"format"`
	Options ollamaSettings `json:"options"`
}

type ollamaSettings struct {
	Temperature float64 `json:"temperature"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Classify sends the excerpt to Ollama and parses the structured verdict.
func (o *Ollama) Classify(ctx context.Context, req Request) (verdict.Verdict, error) {
	prompt := BuildPrompt(req)

	v, err := o.once(ctx, prompt)
	if err == nil {
		return v, nil
	}

	// One retry, and only for a formatting failure. A transport error is not
	// going to be fixed by asking the model more firmly.
	if !isMalformed(err) {
		return verdict.Verdict{}, err
	}

	v, retryErr := o.once(ctx, prompt+"\n\n"+RepairPrompt)
	if retryErr != nil {
		return verdict.Verdict{}, fmt.Errorf("ollama returned unparseable output twice: %w", retryErr)
	}
	return v, nil
}

func (o *Ollama) once(ctx context.Context, prompt string) (verdict.Verdict, error) {
	payload, err := json.Marshal(ollamaRequest{
		Model:  o.model,
		Prompt: prompt,
		System: SystemPrompt,
		Stream: false,
		// Ollama's JSON mode constrains decoding to valid JSON, which removes
		// most of the malformed-response problem at the source.
		Format:  "json",
		Options: ollamaSettings{Temperature: o.temperature},
	})
	if err != nil {
		return verdict.Verdict{}, fmt.Errorf("encoding ollama request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.baseURL+"/api/generate", bytes.NewReader(payload))
	if err != nil {
		return verdict.Verdict{}, fmt.Errorf("building ollama request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return verdict.Verdict{}, fmt.Errorf("calling ollama at %s (is `ollama serve` running?): %w", o.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return verdict.Verdict{}, fmt.Errorf("ollama returned HTTP %d", resp.StatusCode)
	}

	var out ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return verdict.Verdict{}, fmt.Errorf("decoding ollama response: %w", err)
	}

	return ParseVerdict(out.Response)
}

func isMalformed(err error) bool {
	return err != nil && errorsIs(err, ErrMalformed)
}
