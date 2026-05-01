//go:build !(darwin && arm64) || nomlx

package cmd

import "dappco.re/go/ml"

// createServeBackend returns the default HTTP backend for non-MLX builds.
//
//	backend, err := createServeBackend("")
func createServeBackend(_ string) (ml.Backend, error) {
	return ml.NewHTTPBackend(apiURL, modelName), nil
}
