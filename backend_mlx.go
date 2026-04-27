// SPDX-Licence-Identifier: EUPL-1.2

//go:build darwin && arm64 && !nomlx

package ml

import (
	"log/slog"

	"dappco.re/go/inference"
	coreerr "dappco.re/go/log"
	"dappco.re/go/mlx" // registers "metal" backend via init() + Set*Limit
)

// SetMLXMemoryLimits applies Metal cache and memory hard limits before the
// next call to NewMLXBackend / inference.LoadModel. Pass zero for either
// argument to leave that limit untouched. Spec §2.2 — memory management
// before loading.
//
//	ml.SetMLXMemoryLimits(4<<30, 32<<30) // 4 GB cache, 32 GB hard cap
//	ml.SetMLXMemoryLimits(0, 96<<30)     // memory only
func SetMLXMemoryLimits(cacheLimit, memoryLimit uint64) {
	if cacheLimit > 0 {
		mlx.SetCacheLimit(cacheLimit)
	}
	if memoryLimit > 0 {
		mlx.SetMemoryLimit(memoryLimit)
	}
}

// NewMLXBackend loads a model via go-inference's Metal backend and wraps it
// in an InferenceAdapter for use as ml.Backend / ml.StreamingBackend.
//
// The named import of go-mlx registers the "metal" backend so
// inference.LoadModel() automatically uses Metal on Apple Silicon. Load
// options (context length, parallel slots, etc.) are forwarded directly to
// go-inference. Spec §2.2.
//
// Callers that need explicit Metal memory control should call
// ml.SetMLXMemoryLimits before NewMLXBackend; between probes use
// runtime.GC() to release unmanaged caches.
//
//	ml.SetMLXMemoryLimits(4<<30, 32<<30)
//	adapter, err := ml.NewMLXBackend("/models/gemma3-1b",
//	    inference.WithContextLen(8192),
//	)
func NewMLXBackend(modelPath string, loadOpts ...inference.LoadOption) (*InferenceAdapter, error) {
	slog.Info("mlx: loading model via go-inference", "path", modelPath)

	m, err := inference.LoadModel(modelPath, loadOpts...)
	if err != nil {
		return nil, coreerr.E("ml.NewMLXBackend", "mlx", err)
	}

	info := m.Info()
	slog.Info("mlx: model loaded",
		"arch", info.Architecture,
		"layers", info.NumLayers,
		"quant", info.QuantBits,
	)

	return NewInferenceAdapter(m, "mlx"), nil
}
