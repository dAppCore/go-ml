//go:build darwin && arm64 && !nomlx

package cmd

func init() {
	mlCmd.AddCommand(chatCmd)
}
