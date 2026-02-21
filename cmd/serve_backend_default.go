//go:build !(darwin && arm64)

package cmd

import "forge.lthn.ai/core/go-ml"

func createServeBackend() (ml.Backend, error) {
	return ml.NewHTTPBackend(apiURL, modelName), nil
}
