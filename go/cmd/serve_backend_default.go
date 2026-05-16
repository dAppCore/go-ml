//go:build !(darwin && arm64) || nomlx

package cmd

import (
	"dappco.re/go"
	"dappco.re/go/ml"
)

// createServeBackend returns the default HTTP backend for non-MLX builds.
//
//	result := createServeBackend("")
func createServeBackend(_ string) core.Result {
	return core.Ok(ml.NewHTTPBackend(apiURL, modelName))
}
