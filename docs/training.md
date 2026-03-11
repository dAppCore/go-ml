---
title: Training Pipeline
description: Data export, LoRA adapter conversion, training data management, and approval workflows.
---

# Training Pipeline

go-ml manages the training data lifecycle: ingesting raw responses, exporting train/valid/test splits in chat JSONL format, converting MLX LoRA adapters to PEFT and GGUF formats, and scoring training checkpoints.

## Data Export

**File**: `export.go`

Training data uses the chat JSONL format expected by MLX LoRA fine-tuning:

```json
{"messages": [{"role": "user", "content": "..."}, {"role": "assistant", "content": "..."}]}
```

The types:

```go
type ChatMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type TrainingExample struct {
    Messages []ChatMessage `json:"messages"`
}
```

### Filtering and Splitting

`FilterResponses` removes responses that are empty, start with "ERROR:", or are shorter than 50 characters:

```go
filtered := ml.FilterResponses(responses)
```

`SplitData` shuffles with a deterministic seed and splits into train/valid/test by percentage:

```go
train, valid, test := ml.SplitData(filtered, 80, 10, 10, 42)
```

`ValidatePercentages` checks that the split percentages are non-negative and sum to 100.

### Writing JSONL

`WriteTrainingJSONL` writes responses in chat format:

```go
ml.WriteTrainingJSONL("train.jsonl", trainResponses)
```

## Adapter Conversion

go-ml converts MLX LoRA adapters to both HuggingFace PEFT format (for Ollama) and GGUF format (for llama.cpp).

### MLX to PEFT

**File**: `convert.go`

Converts an MLX safetensors adapter to HuggingFace PEFT format:

```go
err := ml.ConvertMLXtoPEFT(
    "adapters/adapter_model.safetensors",  // MLX adapter weights
    "adapters/adapter_config.json",        // MLX adapter config
    "output/peft/",                        // output directory
    "google/gemma-3-1b-it",               // base model name
)
```

The conversion:
1. Reads the MLX safetensors file and parses tensor metadata
2. Renames tensor keys from MLX format (`model.layers.0.self_attn.q_proj.lora_a`) to PEFT format (`base_model.model.model.layers.0.self_attn.q_proj.lora_A.default.weight`)
3. Transposes 2D weight matrices from row-major to column-major
4. Generates `adapter_config.json` with LoRA rank, alpha, target modules, and layer indices
5. Writes the converted safetensors and config to the output directory

Supports F32, F16, and BF16 tensor dtypes.

### MLX to GGUF

**File**: `gguf.go`

Converts an MLX LoRA adapter directly to GGUF v3 format for use with llama.cpp:

```go
err := ml.ConvertMLXtoGGUFLoRA(
    "adapters/adapter_model.safetensors",
    "adapters/adapter_config.json",
    "output/adapter.gguf",
    "gemma3",  // architecture
)
```

The GGUF writer:
1. Maps MLX tensor names to GGUF names (e.g., `self_attn.q_proj` becomes `attn_q`)
2. Transposes weight matrices
3. Writes a GGUF v3 file with adapter metadata (type, architecture, LoRA alpha)
4. Aligns tensor data to 32-byte boundaries

**Architecture mapping**: `ModelTagToGGUFArch` maps model tags to GGUF architecture names. Currently all Gemma 3 variants map to `"gemma3"`.

### Safetensors I/O

**File**: `convert.go`

Low-level safetensors reading and writing:

```go
// Read
tensors, tensorData, err := ml.ReadSafetensors("adapter_model.safetensors")

// Access tensor data
for name, info := range tensors {
    data := ml.GetTensorData(info, tensorData)
    fmt.Printf("%s: dtype=%s shape=%v bytes=%d\n", name, info.Dtype, info.Shape, len(data))
}

// Write
err := ml.WriteSafetensors("output.safetensors", tensors, tensorDataMap)
```

Transpose helpers: `TransposeFloat32`, `TransposeFloat16`, `TransposeBFloat16`.

## Seed Normalisation

**File**: `normalize.go`

`NormalizeSeeds` deduplicates raw seeds into the `expansion_prompts` table:

```go
err := ml.NormalizeSeeds(db, ml.NormalizeConfig{MinLength: 50}, os.Stdout)
```

Steps:
1. Verifies the seeds table exists
2. Deduplicates seeds, excluding prompts already in `prompts` or `golden_set`
3. Assigns priority based on domain coverage (underrepresented domains rank higher)
4. Prints a region distribution summary

## Approval Workflow

**File**: `approve.go`

`ApproveExpansions` filters scored expansion responses and writes approved examples to a training JSONL file:

```go
err := ml.ApproveExpansions(db, ml.ApproveConfig{
    Output:    "approved.jsonl",
    Threshold: 0.0,
}, os.Stdout)
```

Approved responses must pass heuristic scoring and either pass judge scoring or have not yet been judged. Each approved row is written as a chat-format JSONL line.

## Coverage Analysis

**File**: `coverage.go`

`PrintCoverage` analyses seed coverage by region and domain:

```go
err := ml.PrintCoverage(db, os.Stdout)
```

Produces a report with:
- Region distribution with bar chart visualisation
- Top and bottom 10 domains by seed count
- Gap recommendations for underrepresented languages

## Data Consolidation

**File**: `consolidate.go`

`Consolidate` pulls JSONL response files from a remote machine via SSH, merges them by index, deduplicates, and writes a single merged file:

```go
err := ml.Consolidate(ml.ConsolidateConfig{
    M3Host:    "m3-ultra",
    RemoteDir: "/data/responses",
    Pattern:   "gold-*.jsonl",
    OutputDir: "responses",
    MergedOut: "gold-merged.jsonl",
}, os.Stdout)
```

## Publishing

**File**: `publish.go`

`Publish` uploads Parquet training files to HuggingFace Hub:

```go
err := ml.Publish(ml.PublishConfig{
    InputDir: "parquet/",
    Repo:     "lethean/lem-golden-set",
    Public:   false,
    DryRun:   true,
}, os.Stdout)
```

Looks for `train.parquet`, `valid.parquet`, `test.parquet` in the input directory, plus an optional `dataset_card.md` (uploaded as `README.md`). Token is resolved from the config, `HF_TOKEN` environment variable, or `~/.huggingface/token`.

## Parquet I/O

**File**: `parquet.go`

Reads and writes training data in Apache Parquet format for efficient columnar storage and compatibility with HuggingFace datasets. Training examples are stored with their chat message structure preserved.

## InfluxDB Metrics

**File**: `metrics.go`, `influx.go`

`PushMetrics` writes golden set statistics to InfluxDB:

```go
err := ml.PushMetrics(db, influx, os.Stdout)
```

Publishes three measurement types:
- `golden_set_stats`: Overall totals, domain count, voice count, completion percentage
- `golden_set_domain`: Per-domain counts and average generation time
- `golden_set_voice`: Per-voice counts, average characters, average generation time
