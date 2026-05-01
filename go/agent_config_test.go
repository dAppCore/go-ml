package ml

import (
	"dappco.re/go"
)

func TestAgentConfig_AdapterMeta_Good(t *core.T) {
	tag, prefix, stem := AdapterMeta("adapters-27b-reasoning")
	core.AssertEqual(t, "gemma-3-27b", tag)
	core.AssertEqual(t, "G27-reasoning", prefix)
	core.AssertEqual(t, "27b-reasoning", stem)
}

func TestAgentConfig_AdapterMeta_Bad(t *core.T) {
	tag, prefix, stem := AdapterMeta("adapters-unknownmodel")
	core.AssertEqual(t, "unknownmodel", tag)
	core.AssertEqual(t, "unknownmod", prefix)
	core.AssertEqual(t, "unknownmodel", stem)
}

func TestAgentConfig_AdapterMeta_Ugly(t *core.T) {
	tag, prefix, stem := AdapterMeta("15k/gemma-3-1b-creative")
	core.AssertEqual(t, "gemma-3-1b", tag)
	core.AssertContains(t, prefix, "G1")
	core.AssertContains(t, stem, "gemma-3-1b")
}
