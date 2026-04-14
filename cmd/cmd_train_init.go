//go:build darwin && arm64 && !nomlx && cliv1

package cmd

func init() {
	mlCmd.AddCommand(trainCmd)
}
