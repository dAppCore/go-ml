package cmd

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
)

// addGGUFCommand registers `ml gguf` — converts an MLX safetensors LoRA
// adapter to GGUF v3 format for use with llama.cpp.
//
//	core ml gguf --input adapter.safetensors --config adapter_config.json --output adapter.gguf
func addGGUFCommand(c *core.Core) {
	c.Command("ml/gguf", core.Command{
		Description: "Convert MLX LoRA adapter to GGUF format",
		Action: func(opts core.Options) core.Result {
			input := opts.String("input")
			cfgPath := opts.String("config")
			output := opts.String("output")
			arch := optStringOr(opts, "arch", "gemma3")

			if input == "" || cfgPath == "" || output == "" {
				return resultFromError(coreerr.E("cmd.runGGUF", "--input, --config, and --output are required", nil))
			}

			if err := ml.ConvertMLXtoGGUFLoRA(input, cfgPath, output, arch); err != nil {
				return resultFromError(coreerr.E("cmd.runGGUF", "convert to GGUF", err))
			}
			core.Print(nil, "GGUF LoRA adapter written to %s", output)
			return core.Result{OK: true}
		},
	})
}
