package cmd

import (
	"dappco.re/go/core"
<<<<<<< HEAD

=======
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	coreerr "dappco.re/go/core/log"
	"dappco.re/go/core/ml"
	"dappco.re/go/core/cli/pkg/cli"
)

var (
	ggufInput  string
	ggufConfig string
	ggufOutput string
	ggufArch   string
)

var ggufCmd = &cli.Command{
	Use:   "gguf",
	Short: "Convert MLX LoRA adapter to GGUF format",
	Long:  "Converts an MLX safetensors LoRA adapter to GGUF v3 format for use with llama.cpp.",
	RunE:  runGGUF,
}

func init() {
	ggufCmd.Flags().StringVar(&ggufInput, "input", "", "Input safetensors file (required)")
	ggufCmd.Flags().StringVar(&ggufConfig, "config", "", "Adapter config JSON (required)")
	ggufCmd.Flags().StringVar(&ggufOutput, "output", "", "Output GGUF file (required)")
	ggufCmd.Flags().StringVar(&ggufArch, "arch", "gemma3", "GGUF architecture name")
	ggufCmd.MarkFlagRequired("input")
	ggufCmd.MarkFlagRequired("config")
	ggufCmd.MarkFlagRequired("output")
}

func runGGUF(cmd *cli.Command, args []string) error {
	if err := ml.ConvertMLXtoGGUFLoRA(ggufInput, ggufConfig, ggufOutput, ggufArch); err != nil {
		return coreerr.E("cmd.runGGUF", "convert to GGUF", err)
	}
<<<<<<< HEAD
	core.Print(nil,("GGUF LoRA adapter written to %s\n", ggufOutput)
=======
	core.Print(cmd.OutOrStdout(), "GGUF LoRA adapter written to %s", ggufOutput)
>>>>>>> ffb3bef466fdbb5fb407655caa4078c6901f94aa
	return nil
}
