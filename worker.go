package ml

import (
	"net/http"
	"runtime"
	"time"

	"dappco.re/go"
	coreio "dappco.re/go/io"
	coreerr "dappco.re/go/log"
)

// WorkerConfig holds the worker's runtime configuration.
type WorkerConfig struct {
	APIBase      string
	WorkerID     string
	Name         string
	APIKey       string
	GPUType      string
	VRAMGb       int
	Languages    []string
	Models       []string
	InferURL     string
	TaskType     string
	BatchSize    int
	PollInterval time.Duration
	OneShot      bool
	DryRun       bool
}

// APITask represents a task from the LEM API.
type APITask struct {
	ID         int    `json:"id"`
	TaskType   string `json:"task_type"`
	Status     string `json:"status"`
	Language   string `json:"language"`
	Domain     string `json:"domain"`
	ModelName  string `json:"model_name"`
	PromptID   string `json:"prompt_id"`
	PromptText string `json:"prompt_text"`
	Config     *struct {
		Temperature float64 `json:"temperature,omitempty"`
		MaxTokens   int     `json:"max_tokens,omitempty"`
	} `json:"config"`
	Priority int `json:"priority"`
}

// RunWorkerLoop is the main worker loop that polls for tasks and processes them.
func RunWorkerLoop(cfg *WorkerConfig) {
	core.Print(nil, "LEM Worker starting")
	core.Print(nil, "  ID:       %s", cfg.WorkerID)
	core.Print(nil, "  Name:     %s", cfg.Name)
	core.Print(nil, "  API:      %s", cfg.APIBase)
	core.Print(nil, "  Infer:    %s", cfg.InferURL)
	core.Print(nil, "  GPU:      %s (%d GB)", cfg.GPUType, cfg.VRAMGb)
	core.Print(nil, "  Langs:    %v", cfg.Languages)
	core.Print(nil, "  Models:   %v", cfg.Models)
	core.Print(nil, "  Batch:    %d", cfg.BatchSize)
	core.Print(nil, "  Dry-run:  %v", cfg.DryRun)

	if err := workerRegister(cfg); err != nil {
		core.Print(nil, "Registration failed: %v", err)
	}
	core.Print(nil, "Registered with LEM API")

	for {
		processed := workerPoll(cfg)

		if cfg.OneShot {
			core.Print(nil, "One-shot mode: processed %d tasks, exiting", processed)
			return
		}

		if processed == 0 {
			core.Print(nil, "No tasks available, sleeping %v", cfg.PollInterval)
			time.Sleep(cfg.PollInterval)
		}

		workerHeartbeat(cfg)
	}
}

func workerRegister(cfg *WorkerConfig) error {
	body := map[string]any{
		"worker_id": cfg.WorkerID,
		"name":      cfg.Name,
		"version":   "0.1.0",
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
	}
	if cfg.GPUType != "" {
		body["gpu_type"] = cfg.GPUType
	}
	if cfg.VRAMGb > 0 {
		body["vram_gb"] = cfg.VRAMGb
	}
	if len(cfg.Languages) > 0 {
		body["languages"] = cfg.Languages
	}
	if len(cfg.Models) > 0 {
		body["supported_models"] = cfg.Models
	}

	_, err := apiPost(cfg, "/api/lem/workers/register", body)
	return err
}

func workerHeartbeat(cfg *WorkerConfig) {
	body := map[string]any{
		"worker_id": cfg.WorkerID,
	}
	apiPost(cfg, "/api/lem/workers/heartbeat", body)
}

func workerPoll(cfg *WorkerConfig) int {
	url := core.Sprintf("/api/lem/tasks/next?worker_id=%s&limit=%d", cfg.WorkerID, cfg.BatchSize)
	if cfg.TaskType != "" {
		url += "&type=" + cfg.TaskType
	}

	resp, err := apiGet(cfg, url)
	if err != nil {
		core.Print(nil, "Error fetching tasks: %v", err)
		return 0
	}

	var result struct {
		Tasks []APITask `json:"tasks"`
		Count int       `json:"count"`
	}
	if r := core.JSONUnmarshal(resp, &result); !r.OK {
		core.Print(nil, "Error parsing tasks: %v", r.Value)
		return 0
	}

	if result.Count == 0 {
		return 0
	}

	core.Print(nil, "Got %d tasks", result.Count)
	processed := 0

	for _, task := range result.Tasks {
		if err := workerProcessTask(cfg, task); err != nil {
			core.Print(nil, "Task %d failed: %v", task.ID, err)
			apiDelete(cfg, core.Sprintf("/api/lem/tasks/%d/claim", task.ID), map[string]any{
				"worker_id": cfg.WorkerID,
			})
			continue
		}
		processed++
	}

	return processed
}

func workerProcessTask(cfg *WorkerConfig, task APITask) error {
	core.Print(nil, "Processing task %d: %s [%s/%s] %d chars prompt",
		task.ID, task.TaskType, task.Language, task.Domain, len(task.PromptText))

	_, err := apiPost(cfg, core.Sprintf("/api/lem/tasks/%d/claim", task.ID), map[string]any{
		"worker_id": cfg.WorkerID,
	})
	if err != nil {
		return coreerr.E("ml.workerProcessTask", "claim", err)
	}

	apiPatch(cfg, core.Sprintf("/api/lem/tasks/%d/status", task.ID), map[string]any{
		"worker_id": cfg.WorkerID,
		"status":    "in_progress",
	})

	if cfg.DryRun {
		core.Print(nil, "  [DRY-RUN] Would generate response for: %.80s...", task.PromptText)
		return nil
	}

	start := time.Now()
	response, err := workerInfer(cfg, task)
	genTime := time.Since(start)

	if err != nil {
		apiPatch(cfg, core.Sprintf("/api/lem/tasks/%d/status", task.ID), map[string]any{
			"worker_id": cfg.WorkerID,
			"status":    "abandoned",
		})
		return coreerr.E("ml.workerProcessTask", "inference", err)
	}

	modelUsed := task.ModelName
	if modelUsed == "" {
		modelUsed = "default"
	}

	_, err = apiPost(cfg, core.Sprintf("/api/lem/tasks/%d/result", task.ID), map[string]any{
		"worker_id":     cfg.WorkerID,
		"response_text": response,
		"model_used":    modelUsed,
		"gen_time_ms":   int(genTime.Milliseconds()),
	})
	if err != nil {
		return coreerr.E("ml.workerProcessTask", "submit result", err)
	}

	core.Print(nil, "  Completed: %d chars in %v", len(response), genTime.Round(time.Millisecond))
	return nil
}

func workerInfer(cfg *WorkerConfig, task APITask) (string, error) {
	messages := []map[string]string{
		{"role": "user", "content": task.PromptText},
	}

	temp := 0.7
	maxTokens := 2048
	if task.Config != nil {
		if task.Config.Temperature > 0 {
			temp = task.Config.Temperature
		}
		if task.Config.MaxTokens > 0 {
			maxTokens = task.Config.MaxTokens
		}
	}

	reqBody := map[string]any{
		"model":       task.ModelName,
		"messages":    messages,
		"temperature": temp,
		"max_tokens":  maxTokens,
	}

	data := []byte(core.JSONMarshalString(reqBody))

	req, err := http.NewRequest("POST", cfg.InferURL+"/v1/chat/completions", core.NewBuffer(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", coreerr.E("ml.workerInfer", "inference request", err)
	}
	defer resp.Body.Close()

	body, err := readAll(resp.Body)
	if err != nil {
		return "", coreerr.E("ml.workerInfer", "read response", err)
	}

	if resp.StatusCode != 200 {
		return "", coreerr.E("ml.workerInfer", core.Sprintf("inference HTTP %d: %s", resp.StatusCode, truncStr(string(body), 200)), nil)
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if r := core.JSONUnmarshal(body, &chatResp); !r.OK {
		return "", coreerr.E("ml.workerInfer", "parse response", r.Value.(error))
	}

	if len(chatResp.Choices) == 0 {
		return "", coreerr.E("ml.workerInfer", "no choices in response", nil)
	}

	content := chatResp.Choices[0].Message.Content
	if len(content) < 10 {
		return "", coreerr.E("ml.workerInfer", core.Sprintf("response too short: %d chars", len(content)), nil)
	}

	return content, nil
}

// HTTP helpers for the LEM API.

func apiGet(cfg *WorkerConfig, path string) ([]byte, error) {
	req, err := http.NewRequest("GET", cfg.APIBase+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := readAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, coreerr.E("ml.apiGet", core.Sprintf("HTTP %d: %s", resp.StatusCode, truncStr(string(body), 200)), nil)
	}

	return body, nil
}

func apiPost(cfg *WorkerConfig, path string, data map[string]any) ([]byte, error) {
	return apiRequest(cfg, "POST", path, data)
}

func apiPatch(cfg *WorkerConfig, path string, data map[string]any) ([]byte, error) {
	return apiRequest(cfg, "PATCH", path, data)
}

func apiDelete(cfg *WorkerConfig, path string, data map[string]any) ([]byte, error) {
	return apiRequest(cfg, "DELETE", path, data)
}

func apiRequest(cfg *WorkerConfig, method, path string, data map[string]any) ([]byte, error) {
	jsonData := []byte(core.JSONMarshalString(data))

	req, err := http.NewRequest(method, cfg.APIBase+path, core.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := readAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, coreerr.E("ml.apiRequest", core.Sprintf("HTTP %d: %s", resp.StatusCode, truncStr(string(body), 200)), nil)
	}

	return body, nil
}

// MachineID returns the machine ID from /etc/machine-id or hostname fallback.
func MachineID() string {
	if data, err := coreio.Local.Read("/etc/machine-id"); err == nil {
		id := core.Trim(data)
		if len(id) > 0 {
			return id
		}
	}
	h, _ := hostname()
	return h
}

// Hostname returns the system hostname.
func Hostname() string {
	h, _ := hostname()
	return h
}

// ReadKeyFile reads the LEM API key from ~/.config/lem/api_key.
func ReadKeyFile() string {
	home, _ := userHomeDir()
	path := core.Path(home, ".config", "lem", "api_key")
	data, err := coreio.Local.Read(path)
	if err != nil {
		return ""
	}
	return core.Trim(data)
}

// SplitComma splits a comma-separated string into trimmed parts.
func SplitComma(s string) []string {
	var result []string
	for _, part := range core.Split(s, ",") {
		trimmed := core.Trim(part)
		if len(trimmed) > 0 {
			result = append(result, trimmed)
		}
	}
	return result
}

func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
