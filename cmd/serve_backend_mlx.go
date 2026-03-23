//go:build darwin && arm64 && !nomlx

package cmd

import (
	"log/slog"

	"forge.lthn.ai/core/go-ml"

	coreerr "forge.lthn.ai/core/go-log"
)

func createServeBackend() (ml.Backend, error) {
	if serveModelPath != "" {
		slog.Info("ml serve: loading native MLX backend", "path", serveModelPath)
		b, err := ml.NewMLXBackend(serveModelPath)
		if err != nil {
			return nil, coreerr.E("cmd.createServeBackend", "mlx backend", err)
		}
		return b, nil
	}
	return ml.NewHTTPBackend(apiURL, modelName), nil
}
