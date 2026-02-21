//go:build darwin && arm64

package cmd

func init() {
	mlCmd.AddCommand(lessonCmd)
	mlCmd.AddCommand(sequenceCmd)
}
