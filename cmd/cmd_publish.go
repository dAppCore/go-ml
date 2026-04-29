package cmd

import (
	"dappco.re/go"
	coreerr "dappco.re/go/log"
	"dappco.re/go/store"
)

// addPublishCommand registers `ml publish` — uploads train/valid/test Parquet
// files and an optional dataset card to a HuggingFace dataset repository.
//
//	core ml publish --input-dir ./parquet --repo lthn/LEM-golden-set --public
func addPublishCommand(c *core.Core) {
	c.Command("ml/publish", core.Command{
		Description: "Upload Parquet dataset to HuggingFace Hub",
		Action: func(opts core.Options) core.Result {
			inputDir := opts.String("input-dir")
			if inputDir == "" {
				return resultFromError(coreerr.E("cmd.runPublish", "--input-dir is required", nil))
			}
			return resultFromError(store.Publish(store.PublishConfig{
				InputDir: inputDir,
				Repo:     optStringOr(opts, "repo", "lthn/LEM-golden-set"),
				Public:   opts.Bool("public"),
				Token:    opts.String("token"),
				DryRun:   opts.Bool("dry-run"),
			}, nil))
		},
	})
}
