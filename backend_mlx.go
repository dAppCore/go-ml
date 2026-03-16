// SPDX-Licence-Identifier: EUPL-1.2

//go:build darwin && arm64

package ml

import (
	"log/slog"

	"forge.lthn.ai/core/go-inference"
	_ "forge.lthn.ai/core/go-mlx" // registers "metal" backend via init()

	coreerr "forge.lthn.ai/core/go-log"
)

// NewMLXBackend loads a model via go-inference's Metal backend and wraps it
// in an InferenceAdapter for use as ml.Backend/StreamingBackend.
//
// The blank import of go-mlx registers the "metal" backend, so
// inference.LoadModel() will automatically use Metal on Apple Silicon.
//
// Load options (context length, etc.) are forwarded directly to go-inference.
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
