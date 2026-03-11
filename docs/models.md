---
title: Model Management
description: GGUF model parsing, Ollama integration, checkpoint discovery, and the scoring agent.
---

# Model Management

go-ml provides GGUF format handling, Ollama model lifecycle management, remote checkpoint discovery via SSH, and an automated scoring agent that evaluates training checkpoints.

## GGUF Format

**File**: `gguf.go`

### Writing GGUF Files

The GGUF writer produces v3-format files with 32-byte alignment. This is primarily used for converting MLX LoRA adapters to GGUF format (see [Training Pipeline](training.md#mlx-to-gguf)).

### Tensor Name Mapping

`MLXTensorToGGUF` converts MLX tensor names to GGUF equivalents:

```
model.layers.0.self_attn.q_proj.lora_a  ->  blk.0.attn_q.weight.lora_a
model.layers.5.mlp.gate_proj.lora_b     ->  blk.5.ffn_gate.weight.lora_b
```

The module mapping (Gemma 3 architecture):

| HuggingFace Module | GGUF Tensor |
|-------------------|-------------|
| `self_attn.q_proj` | `attn_q` |
| `self_attn.k_proj` | `attn_k` |
| `self_attn.v_proj` | `attn_v` |
| `self_attn.o_proj` | `attn_output` |
| `mlp.gate_proj` | `ffn_gate` |
| `mlp.up_proj` | `ffn_up` |
| `mlp.down_proj` | `ffn_down` |

### Ollama Model Blob Resolution

`GGUFModelBlobPath` resolves an Ollama model name to the GGUF blob path on disk:

```go
path, err := ml.GGUFModelBlobPath("~/.ollama/models", "gemma3:1b")
// -> ~/.ollama/models/blobs/sha256-abc123...
```

Reads the Ollama manifest file, finds the layer with media type `application/vnd.ollama.image.model`, and returns the blob path.

## Ollama Integration

**File**: `ollama.go`

### Creating Models with LoRA Adapters

`OllamaCreateModel` creates a temporary Ollama model by uploading adapter weights and config as blobs:

```go
err := ml.OllamaCreateModel(
    "http://localhost:11434",  // Ollama URL
    "lem-gemma-3-1b-500",     // temporary model name
    "gemma3:1b",              // base model
    "/tmp/peft-adapter/",     // directory with adapter_model.safetensors + adapter_config.json
)
```

The process:
1. Uploads `adapter_model.safetensors` to Ollama's blob store (skips if digest already exists)
2. Uploads `adapter_config.json` to the blob store
3. Calls `POST /api/create` with the base model and adapter blob digests
4. Streams the creation status until "success"

### Deleting Models

```go
err := ml.OllamaDeleteModel("http://localhost:11434", "lem-gemma-3-1b-500")
```

### Model Maps

```go
// Model tag -> Ollama model name
ml.OllamaBaseModelMap["gemma-3-1b"]   // "gemma3:1b"
ml.OllamaBaseModelMap["gemma-3-27b"]  // "gemma3:27b"

// Model tag -> HuggingFace model ID
ml.HFBaseModelMap["gemma-3-1b"]   // "google/gemma-3-1b-it"
ml.HFBaseModelMap["gemma-3-4b"]   // "google/gemma-3-4b-it"

// Model tag -> MLX community model path
ml.BaseModelMap["gemma-3-1b"]   // "mlx-community/gemma-3-1b-it-4bit"
ml.BaseModelMap["gemma-3-27b"]  // "mlx-community/gemma-3-27b-it-qat-4bit"
```

## Checkpoint Discovery

**File**: `agent_execute.go`, `agent_config.go`

The scoring agent discovers training checkpoints on a remote machine (M3) via SSH.

### Checkpoint Structure

```go
type Checkpoint struct {
    RemoteDir string  // full remote path to the adapter directory
    Filename  string  // safetensors filename (e.g., "0000500_adapters.safetensors")
    Dirname   string  // adapter directory name (e.g., "adapters-1b-v2")
    Iteration int     // training iteration extracted from filename
    ModelTag  string  // e.g., "gemma-3-1b"
    Label     string  // human-readable label (e.g., "G1-v2 @500")
    RunID     string  // unique run identifier for deduplication
}
```

### Discovery Process

`DiscoverCheckpoints` (and its iterator variant `DiscoverCheckpointsIter`) scans remote directories:

```go
checkpoints, err := ml.DiscoverCheckpoints(cfg)
```

1. Lists `adapters-*` directories on the remote host
2. Checks for nested model-specific subdirectories (e.g., `adapters-15k/gemma-3-1b/`)
3. Finds `*_adapters.safetensors` files in each directory
4. Extracts iteration numbers from filenames
5. Maps directory names to model tags and labels via `AdapterMeta`

### Model Family Detection

`AdapterMeta` maps adapter directory names to metadata:

```go
tag, label, runID := ml.AdapterMeta("adapters-1b-v2")
// tag:   "gemma-3-1b"
// label: "G1-v2"
// runID: "1b-v2"
```

Supported prefixes: `1b`, `4b`, `12b`, `27b`, `vi`, `gpt-oss`, `bench-1b`, `book`, `cross`, `deepseek-r1-7b`, and nested `15k/gemma-3-*` paths.

## Scoring Agent

**File**: `agent_execute.go`, `agent_eval.go`, `agent_influx.go`

The scoring agent is a continuous loop that discovers unscored checkpoints and evaluates them.

### Agent Loop

```go
ml.RunAgentLoop(&ml.AgentConfig{
    M3Host:        "192.168.1.100",
    M3User:        "user",
    M3SSHKey:      "~/.ssh/id_ed25519",
    M3AdapterBase: "/data/adapters",
    InfluxURL:     "http://10.69.69.165:8181",
    InfluxDB:      "training",
    DBPath:        "scores.duckdb",
    JudgeURL:      "http://localhost:11434",
    JudgeModel:    "qwen3:8b",
    PollInterval:  60,
    WorkDir:       "/tmp/scoring",
})
```

Each iteration:
1. Replays any buffered InfluxDB writes from previous failures
2. Discovers all checkpoints on M3
3. Queries InfluxDB for already-scored `(run_id, label)` pairs
4. Selects the first unscored checkpoint (or all in force mode)
5. Processes the checkpoint via `ProcessOne`

### Checkpoint Processing

`ProcessOne` routes to one of two paths:

**MLX Native** (`processMLXNative`): For Gemma and GPT-OSS models:
1. Fetches adapter files from M3 via SCP
2. Converts MLX to PEFT format
3. Creates a temporary Ollama model with the adapter
4. Runs 23 capability probes with real-time InfluxDB streaming
5. Runs LLM judge scoring on capability responses (reasoning, correctness, clarity)
6. Runs 6 content probes with judge scoring (sovereignty dimensions)
7. Cleans up temporary files and Ollama model

**With Conversion** (`processWithConversion`): For other architectures:
1. Fetches adapter, converts MLX to PEFT
2. Runs capability probes against a remote inference API
3. Pushes results to InfluxDB and DuckDB

### Result Storage

Results are published to two backends:

**InfluxDB** (time-series): Measurements include `capability_score` (overall + per-category), `probe_score` (per-probe pass/fail), `capability_judge` (per-probe quality scores), and `content_score` (per-dimension content scores). Timestamps are derived from `EpochBase + iteration * 1000` for correct time-series ordering.

**DuckDB** (persistent): Tables `checkpoint_scores` and `probe_results` store the same data for durable querying.

If InfluxDB is unreachable, results are buffered to a local JSONL file (`influx_buffer.jsonl`) and replayed on the next loop iteration.

## Remote Transport

**File**: `agent_ssh.go`

The `RemoteTransport` interface abstracts remote command execution:

```go
type RemoteTransport interface {
    Run(ctx context.Context, cmd string) (string, error)
    CopyFrom(ctx context.Context, remote, local string) error
    CopyTo(ctx context.Context, local, remote string) error
}
```

`SSHTransport` implements this using the `ssh` and `scp` binaries:

```go
transport := ml.NewSSHTransport("192.168.1.100", "user", "~/.ssh/key",
    ml.WithPort("22"),
    ml.WithTimeout(10 * time.Second),
)

output, err := transport.Run(ctx, "ls /data/adapters")
err = transport.CopyFrom(ctx, "/remote/file", "/local/file")
```

Uses `BatchMode=yes` and `StrictHostKeyChecking=no` for non-interactive operation.

## DuckDB Storage

**File**: `db.go`

`DB` wraps a DuckDB connection for training data and scoring results:

```go
// Read-only (avoids locking the Python pipeline)
db, err := ml.OpenDB("training.duckdb")

// Read-write (for scoring agent)
db, err := ml.OpenDBReadWrite("scores.duckdb")
defer db.Close()
```

### Key Tables

| Table | Purpose |
|-------|---------|
| `golden_set` | Curated training examples with domain, voice, response |
| `expansion_prompts` | Deduplicated seeds awaiting generation |
| `seeds` | Raw imported seed prompts |
| `prompts` | Processed prompts |
| `training_examples` | Approved training examples |
| `gemini_responses` | Responses from Gemini models |
| `benchmark_questions` | Industry benchmark questions |
| `benchmark_results` | Benchmark scoring results |
| `checkpoint_scores` | Per-checkpoint accuracy (PK: run_id, label) |
| `probe_results` | Per-probe pass/fail results (PK: run_id, label, probe_id) |
| `scoring_results` | Arbitrary scoring dimension results |

### Inventory

`PrintInventory` produces a formatted table of all DuckDB tables with row counts and contextual annotations:

```go
ml.PrintInventory(db, os.Stdout)
```

## Status Reporting

**File**: `status.go`

`PrintStatus` queries InfluxDB for real-time training and generation progress:

```go
influx := ml.NewInfluxClient("http://10.69.69.165:8181", "training")
ml.PrintStatus(influx, os.Stdout)
```

Output:
```
Training:
  gemma-3-1b    running    450/1000  45.0%  loss=1.234

Generation:
  golden        3200/5000  64.0%  (m3-worker)
  expansion     1500/8000  18.8%  (m3-worker)
```

## InfluxDB Client

**File**: `influx.go`

```go
influx := ml.NewInfluxClient("http://10.69.69.165:8181", "training")

// Write line protocol
influx.WriteLp([]string{
    "capability_score,model=gemma-3-1b,label=G1\\ @500 accuracy=87.0 1739577600000000000",
})

// Query with SQL
rows, err := influx.QuerySQL("SELECT * FROM capability_score LIMIT 10")
```

Token is resolved from `INFLUX_TOKEN` environment variable or `~/.influx_token` file. Defaults: URL `http://10.69.69.165:8181`, database `training`.

`EscapeLp` escapes spaces, commas, and equals signs for InfluxDB line protocol tag values.
