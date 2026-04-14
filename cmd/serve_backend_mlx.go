//go:build darwin && arm64 && !nomlx

package cmd

import (
	"log/slog"

	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
)

// createServeBackend returns the MLX backend when available, falling back to
// HTTP when modelPath is empty.
//
//	backend, err := createServeBackend("/Volumes/Data/lem/models/lem")
func createServeBackend(modelPath string) (ml.Backend, error) {
	if modelPath != "" {
		slog.Info("ml serve: loading native MLX backend", "path", modelPath)
		b, err := ml.NewMLXBackend(modelPath)
		if err != nil {
			return nil, coreerr.E("cmd.createServeBackend", "mlx backend", err)
		}
		return b, nil
	}
	return ml.NewHTTPBackend(apiURL, modelName), nil
}
