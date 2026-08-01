//go:build darwin && arm64 && !nomlx

package cmd

import (
	"log/slog"

	"dappco.re/go"
	"dappco.re/go/ml"
)

// createServeBackend returns the MLX backend when available, falling back to
// HTTP when modelPath is empty.
//
//	result := createServeBackend("/Volumes/Data/lem/models/lem")
func createServeBackend(modelPath string) core.Result {
	if modelPath != "" {
		slog.Info("ml serve: loading native MLX backend", "model_path", modelPath)
		result := ml.NewMLXBackend(modelPath)
		if !result.OK {
			return core.Fail(core.E("cmd.createServeBackend", "mlx backend", result.Value.(error)))
		}
		return result
	}
	return core.Ok(ml.NewHTTPBackend(apiURL, modelName))
}
