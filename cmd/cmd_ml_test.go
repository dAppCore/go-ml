package cmd

import "dappco.re/go"

func TestCmdMl_AddMLCommands_Good(t *core.T) {
	c := core.New()
	AddMLCommands(c)
	result := c.Command("ml")
	core.AssertTrue(t, result.OK)
}

func TestCmdMl_AddMLCommands_Bad(t *core.T) {
	c := core.New()
	AddMLCommands(c)
	result := c.Command("ml/missing")
	core.AssertFalse(t, result.OK)
}

func TestCmdMl_AddMLCommands_Ugly(t *core.T) {
	c := core.New()
	AddMLCommands(c)
	AddMLCommands(c)
	result := c.Command("ml")
	core.AssertTrue(t, result.OK)
}
