package ml

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"sync"

	"forge.lthn.ai/core/go/pkg/core"
)

// Service manages ML inference backends and scoring with Core lifecycle.
type Service struct {
	*core.ServiceRuntime[Options]

	backends map[string]Backend
	mu       sync.RWMutex
	engine   *Engine
	judge    *Judge
}

// Options configures the ML service.
type Options struct {
	// DefaultBackend is the name of the default inference backend.
	DefaultBackend string

	// LlamaPath is the path to the llama-server binary.
	LlamaPath string

	// ModelDir is the directory containing model files.
	ModelDir string

	// OllamaURL is the Ollama API base URL.
	OllamaURL string

	// JudgeURL is the judge model API URL.
	JudgeURL string

	// JudgeModel is the judge model name.
	JudgeModel string

	// InfluxURL is the InfluxDB URL for metrics.
	InfluxURL string

	// InfluxDB is the InfluxDB database name.
	InfluxDB string

	// Concurrency is the number of concurrent scoring workers.
	Concurrency int

	// Suites is a comma-separated list of scoring suites to enable.
	Suites string
}

// NewService creates an ML service factory for Core registration.
//
//	core, _ := core.New(
//	    core.WithName("ml", ml.NewService(ml.Options{})),
//	)
func NewService(opts Options) func(*core.Core) (any, error) {
	return func(c *core.Core) (any, error) {
		if opts.Concurrency == 0 {
			opts.Concurrency = 4
		}
		if opts.Suites == "" {
			opts.Suites = "all"
		}

		svc := &Service{
			ServiceRuntime: core.NewServiceRuntime(c, opts),
			backends:       make(map[string]Backend),
		}
		return svc, nil
	}
}

// OnStartup initializes backends and scoring engine.
func (s *Service) OnStartup(ctx context.Context) error {
	opts := s.Opts()

	// Register Ollama backend if URL provided.
	if opts.OllamaURL != "" {
		s.RegisterBackend("ollama", NewHTTPBackend(opts.OllamaURL, opts.JudgeModel))
	}

	// Set up judge if judge URL is provided.
	if opts.JudgeURL != "" {
		judgeBackend := NewHTTPBackend(opts.JudgeURL, opts.JudgeModel)
		s.judge = NewJudge(judgeBackend)
		s.engine = NewEngine(s.judge, opts.Concurrency, opts.Suites)
	}

	return nil
}

// OnShutdown cleans up resources.
func (s *Service) OnShutdown(ctx context.Context) error {
	return nil
}

// RegisterBackend adds or replaces a named inference backend.
func (s *Service) RegisterBackend(name string, backend Backend) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backends[name] = backend
}

// Backend returns a named backend, or nil if not found.
func (s *Service) Backend(name string) Backend {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backends[name]
}

// DefaultBackend returns the configured default backend.
func (s *Service) DefaultBackend() Backend {
	name := s.Opts().DefaultBackend
	if name == "" {
		name = "ollama"
	}
	return s.Backend(name)
}

// Backends returns the names of all registered backends.
func (s *Service) Backends() []string {
	return slices.Collect(s.BackendsIter())
}

// BackendsIter returns an iterator over the names of all registered backends.
func (s *Service) BackendsIter() iter.Seq[string] {
	return func(yield func(string) bool) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		for name := range s.backends {
			if !yield(name) {
				return
			}
		}
	}
}

// Judge returns the configured judge, or nil if not set up.
func (s *Service) Judge() *Judge {
	return s.judge
}

// Engine returns the scoring engine, or nil if not set up.
func (s *Service) Engine() *Engine {
	return s.engine
}

// Generate generates text using the named backend (or default).
func (s *Service) Generate(ctx context.Context, backendName, prompt string, opts GenOpts) (Result, error) {
	b := s.Backend(backendName)
	if b == nil {
		b = s.DefaultBackend()
	}
	if b == nil {
		return Result{}, fmt.Errorf("no backend available (requested: %q)", backendName)
	}
	return b.Generate(ctx, prompt, opts)
}

// ScoreResponses scores a batch of responses using the configured engine.
func (s *Service) ScoreResponses(ctx context.Context, responses []Response) (map[string][]PromptScore, error) {
	if s.engine == nil {
		return nil, errors.New("scoring engine not configured (set JudgeURL and JudgeModel)")
	}
	return s.engine.ScoreAll(ctx, responses), nil
}
