//go:build !(darwin && arm64)

package cmd

import "dappco.re/go/core/ml"

func createServeBackend() (ml.Backend, error) {
	return ml.NewHTTPBackend(apiURL, modelName), nil
}
