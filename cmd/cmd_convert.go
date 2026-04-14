package cmd

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
)

// addConvertCommand registers `ml convert` — converts an MLX safetensors LoRA
// adapter to HuggingFace PEFT format for consumption by Ollama.
//
//	core ml convert --input adapter.safetensors --config adapter_config.json --output-dir peft/
func addConvertCommand(c *core.Core) {
	c.Command("ml/convert", core.Command{
		Description: "Convert MLX LoRA adapter to PEFT format",
		Action: func(opts core.Options) core.Result {
			input := opts.String("input")
			cfgPath := opts.String("config")
			outputDir := opts.String("output-dir")
			baseModel := opts.String("base-model")

			if input == "" || cfgPath == "" || outputDir == "" {
				return resultFromError(coreerr.E("cmd.runConvert", "--input, --config, and --output-dir are required", nil))
			}

			if err := ml.ConvertMLXtoPEFT(input, cfgPath, outputDir, baseModel); err != nil {
				return resultFromError(coreerr.E("cmd.runConvert", "convert to PEFT", err))
			}
			core.Print(nil, "PEFT adapter written to %s", outputDir)
			return core.Result{OK: true}
		},
	})
}
