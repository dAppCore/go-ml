//go:build darwin && arm64

package cmd

func init() {
	mlCmd.AddCommand(chatCmd)
}
