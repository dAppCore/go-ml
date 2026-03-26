package cmd

import (
	"dappco.re/go/core"
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"forge.lthn.ai/core/cli/pkg/cli"
)

var (
	convertInput     string
	convertConfig    string
	convertOutputDir string
	convertBaseModel string
)

var convertCmd = &cli.Command{
	Use:   "convert",
	Short: "Convert MLX LoRA adapter to PEFT format",
	Long:  "Converts an MLX safetensors LoRA adapter to HuggingFace PEFT format for Ollama.",
	RunE:  runConvert,
}

func init() {
	convertCmd.Flags().StringVar(&convertInput, "input", "", "Input safetensors file (required)")
	convertCmd.Flags().StringVar(&convertConfig, "config", "", "Adapter config JSON (required)")
	convertCmd.Flags().StringVar(&convertOutputDir, "output-dir", "", "Output directory (required)")
	convertCmd.Flags().StringVar(&convertBaseModel, "base-model", "", "Base model name for adapter_config.json")
	convertCmd.MarkFlagRequired("input")
	convertCmd.MarkFlagRequired("config")
	convertCmd.MarkFlagRequired("output-dir")
}

func runConvert(cmd *cli.Command, args []string) error {
	if err := ml.ConvertMLXtoPEFT(convertInput, convertConfig, convertOutputDir, convertBaseModel); err != nil {
		return coreerr.E("cmd.runConvert", "convert to PEFT", err)
	}
	core.Print(cmd.OutOrStdout(), "PEFT adapter written to %s", convertOutputDir)
	return nil
}
