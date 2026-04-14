package ml

import (
	"context"
	"net/http"
	"time"

	"dappco.re/go/core"
	"dappco.re/go/core/inference"
	"dappco.re/go/core/log"
	"dappco.re/go/core/process"
)

// Compile-time check: LlamaBackend satisfies inference.Backend (spec §2.1).
var _ inference.Backend = (*LlamaBackend)(nil)

// LlamaBackend manages a llama-server process and delegates HTTP calls to it.
type LlamaBackend struct {
	processSvc *process.Service
	procID     string
	port       int
	http       *HTTPBackend
	modelPath  string
	loraPath   string
	llamaPath  string
}

// LlamaOpts configures the llama-server backend.
type LlamaOpts struct {
	// LlamaPath is the path to the llama-server binary.
	LlamaPath string
	// ModelPath is the path to the GGUF model file.
	ModelPath string
	// LoraPath is the optional path to a GGUF LoRA adapter file.
	LoraPath string
	// Port is the HTTP port for llama-server (default: 18090).
	Port int
}

// NewLlamaBackend creates a backend that manages a llama-server process.
//
// The constructor accepts the current process-service form used by the codebase
// as well as the RFC's simpler model-path form:
//
//	NewLlamaBackend(processSvc, LlamaOpts{ModelPath: "model.gguf"})
//	NewLlamaBackend("model.gguf")
func NewLlamaBackend(args ...any) *LlamaBackend {
	var processSvc *process.Service
	opts := LlamaOpts{}

	for _, arg := range args {
		switch v := arg.(type) {
		case *process.Service:
			processSvc = v
		case LlamaOpts:
			if v.LlamaPath != "" {
				opts.LlamaPath = v.LlamaPath
			}
			if v.ModelPath != "" {
				opts.ModelPath = v.ModelPath
			}
			if v.LoraPath != "" {
				opts.LoraPath = v.LoraPath
			}
			if v.Port != 0 {
				opts.Port = v.Port
			}
		case string:
			if opts.ModelPath == "" {
				opts.ModelPath = v
			}
		case nil:
			// Allow the existing nil placeholder used by tests and callers.
		}
	}

	if opts.Port == 0 {
		opts.Port = 18090
	}
	if opts.LlamaPath == "" {
		opts.LlamaPath = "llama-server"
	}

	baseURL := core.Sprintf("http://127.0.0.1:%d", opts.Port)
	return &LlamaBackend{
		processSvc: processSvc,
		port:       opts.Port,
		modelPath:  opts.ModelPath,
		loraPath:   opts.LoraPath,
		llamaPath:  opts.LlamaPath,
		http:       NewHTTPBackend(baseURL, ""),
	}
}

// Name returns "llama".
func (b *LlamaBackend) Name() string { return "llama" }

// SetMaxTokens sets the maximum token count forwarded to the managed
// llama-server for subsequent Generate/Chat calls. Matches spec §2.4.
//
//	backend := ml.NewLlamaBackend(process, ml.LlamaOpts{...})
//	backend.SetMaxTokens(2048)
func (b *LlamaBackend) SetMaxTokens(n int) {
	if b.http != nil {
		b.http.SetMaxTokens(n)
	}
}

// LoadModel satisfies inference.Backend by wrapping the managed llama-server
// as an inference.TextModel. The path argument is ignored — the GGUF path is
// supplied at construction time via LlamaOpts.ModelPath. Spec §2.4.
//
//	backend := ml.NewLlamaBackend(svc, ml.LlamaOpts{ModelPath: "model.gguf"})
//	model, _ := backend.LoadModel("dummy")
//	for tok := range model.Generate(ctx, "hello") {
//	    fmt.Print(tok.Text)
//	}
func (b *LlamaBackend) LoadModel(_ string, _ ...inference.LoadOption) (inference.TextModel, error) {
	if b.http == nil {
		return nil, log.E("ml.LlamaBackend.LoadModel", "HTTP shim not configured", nil)
	}
	return NewLlamaTextModel(b), nil
}

// Available checks if the llama-server is responding to health checks.
func (b *LlamaBackend) Available() bool {
	if b.procID == "" {
		return false
	}
	url := core.Sprintf("http://127.0.0.1:%d/health", b.port)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Start launches the llama-server process.
func (b *LlamaBackend) Start(ctx context.Context) error {
	if b.processSvc == nil {
		return log.E("ml.LlamaBackend.Start", "process service not configured", nil)
	}

	args := []string{
		"-m", b.modelPath,
		"--port", core.Sprintf("%d", b.port),
		"--host", "127.0.0.1",
	}
	if b.loraPath != "" {
		args = append(args, "--lora", b.loraPath)
	}

	proc, err := b.processSvc.StartWithOptions(ctx, process.RunOptions{
		Command: b.llamaPath,
		Args:    args,
	})
	if err != nil {
		return log.E("ml.LlamaBackend.Start", "failed to start llama-server", err)
	}
	b.procID = proc.ID

	// Wait for health check (up to 30 seconds).
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if b.Available() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return log.E("ml.LlamaBackend.Start", "llama-server did not become healthy within 30s", nil)
}

// Stop terminates the llama-server process.
func (b *LlamaBackend) Stop() error {
	if b.procID == "" {
		return nil
	}
	if b.processSvc == nil {
		return log.E("ml.LlamaBackend.Stop", "process service not configured", nil)
	}
	return b.processSvc.Kill(b.procID)
}

// Generate sends a prompt to the managed llama-server.
func (b *LlamaBackend) Generate(ctx context.Context, prompt string, opts GenOpts) (Result, error) {
	if !b.Available() {
		return Result{}, log.E("ml.LlamaBackend.Generate", "llama-server not available", nil)
	}
	if b.http == nil {
		return Result{}, log.E("ml.LlamaBackend.Generate", "HTTP shim not configured", nil)
	}
	return b.http.Generate(ctx, prompt, opts)
}

// Chat sends a conversation to the managed llama-server.
func (b *LlamaBackend) Chat(ctx context.Context, messages []Message, opts GenOpts) (Result, error) {
	if !b.Available() {
		return Result{}, log.E("ml.LlamaBackend.Chat", "llama-server not available", nil)
	}
	if b.http == nil {
		return Result{}, log.E("ml.LlamaBackend.Chat", "HTTP shim not configured", nil)
	}
	return b.http.Chat(ctx, messages, opts)
}
