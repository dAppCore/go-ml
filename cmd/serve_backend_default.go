//go:build !(darwin && arm64) || nomlx

package cmd

import "dappco.re/go/core/ml"

// createServeBackend returns the default HTTP backend for non-MLX builds.
//
//	backend, err := createServeBackend("")
func createServeBackend(modelPath string) (ml.Backend, error) {
	_ = modelPath // unused in default (HTTP) backend
	return ml.NewHTTPBackend(apiURL, modelName), nil
}
