// TODO(virgil): Re-enable with cmd_train.go when go-mlx training API is exported.
//go:build ignore

package cmd

func init() {
	mlCmd.AddCommand(trainCmd)
}
