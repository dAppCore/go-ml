// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package main

import (
	"dappco.re/go/cli/pkg/cli"
	"dappco.re/go/ml/cmd"
)

func main() {
	cli.WithAppName("lem")
	cli.Main(
		cli.WithCommands("ml", cmd.AddMLCommands),
	)
}
