package ml

import (
	"dappco.re/go"
	coreio "dappco.re/go/io"
)

func writeOllamaManifest(t *core.T, root string, model string) {
	t.Helper()
	manifest := core.JoinPath(root, "manifests", "registry.ollama.ai", "library", model, "latest")
	core.RequireNoError(t, coreio.Local.EnsureDir(core.PathDir(manifest)))
	core.RequireNoError(t, coreio.Local.Write(manifest, `{"layers":[{"mediaType":"application/vnd.ollama.image.model","digest":"sha256:abc"}]}`))
}

func TestGguf_ReadGGUFInfo_Good(t *core.T) {
	file := core.JoinPath(t.TempDir(), "missing.gguf")
	info, err := ReadGGUFInfo(file)
	core.AssertError(t, err)
	core.AssertEqual(t, "", info.Architecture)
}

func TestGguf_ReadGGUFInfo_Bad(t *core.T) {
	info, err := ReadGGUFInfo("")
	core.AssertError(t, err)
	core.AssertEqual(t, "", info.Architecture)
}

func TestGguf_ReadGGUFInfo_Ugly(t *core.T) {
	dir := t.TempDir()
	info, err := ReadGGUFInfo(dir)
	core.AssertError(t, err)
	core.AssertEqual(t, "", info.Architecture)
}

func TestGguf_DiscoverModels_Good(t *core.T) {
	models := DiscoverModels(t.TempDir())
	core.AssertEmpty(t, models)
	core.AssertEqual(t, 0, len(models))
}

func TestGguf_DiscoverModels_Bad(t *core.T) {
	models := DiscoverModels(core.JoinPath(t.TempDir(), "missing"))
	core.AssertEmpty(t, models)
	core.AssertEqual(t, 0, len(models))
}

func TestGguf_DiscoverModels_Ugly(t *core.T) {
	dir := t.TempDir()
	core.RequireNoError(t, coreio.Local.Write(core.JoinPath(dir, "note.txt"), "not a model"))
	models := DiscoverModels(dir)
	core.AssertEmpty(t, models)
}

func TestGguf_MLXTensorToGGUF_Good(t *core.T) {
	got, err := MLXTensorToGGUF("model.layers.0.self_attn.q_proj.lora_a")
	core.RequireNoError(t, err)
	core.AssertEqual(t, "blk.0.attn_q.weight.lora_a", got)
}

func TestGguf_MLXTensorToGGUF_Bad(t *core.T) {
	got, err := MLXTensorToGGUF("bad.key")
	core.AssertError(t, err)
	core.AssertEqual(t, "", got)
}

func TestGguf_MLXTensorToGGUF_Ugly(t *core.T) {
	got, err := MLXTensorToGGUF("model.layers.2.mlp.down_proj.lora_b")
	core.RequireNoError(t, err)
	core.AssertEqual(t, "blk.2.ffn_down.weight.lora_b", got)
}

func TestGguf_SafetensorsDtypeToGGML_Good(t *core.T) {
	got, err := SafetensorsDtypeToGGML("F32")
	core.RequireNoError(t, err)
	core.AssertEqual(t, uint32(0), got)
}

func TestGguf_SafetensorsDtypeToGGML_Bad(t *core.T) {
	got, err := SafetensorsDtypeToGGML("I8")
	core.AssertError(t, err)
	core.AssertEqual(t, uint32(0), got)
}

func TestGguf_SafetensorsDtypeToGGML_Ugly(t *core.T) {
	got, err := SafetensorsDtypeToGGML("BF16")
	core.RequireNoError(t, err)
	core.AssertEqual(t, uint32(30), got)
}

func TestGguf_ConvertMLXtoGGUFLoRA_Good(t *core.T) {
	sf, cfg := writeSafetensorsFixture(t)
	out := core.JoinPath(t.TempDir(), "adapter.gguf")
	err := ConvertMLXtoGGUFLoRA(sf, cfg, out, "gemma3")
	core.RequireNoError(t, err)
	core.AssertTrue(t, coreio.Local.IsFile(out))
}

func TestGguf_ConvertMLXtoGGUFLoRA_Bad(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	err := ConvertMLXtoGGUFLoRA(core.JoinPath(t.TempDir(), "missing.safetensors"), core.JoinPath(t.TempDir(), "missing.cfg"), core.JoinPath(t.TempDir(), "out.gguf"), "gemma3")
	core.AssertError(t, err)
}

func TestGguf_ConvertMLXtoGGUFLoRA_Ugly(t *core.T) {
	sf, _ := writeSafetensorsFixture(t)
	cfg := core.JoinPath(t.TempDir(), "bad.cfg")
	core.RequireNoError(t, coreio.Local.Write(cfg, "bad"))
	err := ConvertMLXtoGGUFLoRA(sf, cfg, core.JoinPath(t.TempDir(), "out.gguf"), "gemma3")
	core.AssertError(t, err)
}

func TestGguf_DetectArchFromConfig_Good(t *core.T) {
	_, cfg := writeSafetensorsFixture(t)
	got := DetectArchFromConfig(cfg)
	core.AssertEqual(t, "gemma3", got)
}

func TestGguf_DetectArchFromConfig_Bad(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	got := DetectArchFromConfig(core.JoinPath(t.TempDir(), "missing.cfg"))
	core.AssertEqual(t, "gemma3", got)
}

func TestGguf_DetectArchFromConfig_Ugly(t *core.T) {
	cfg := core.JoinPath(t.TempDir(), "empty.cfg")
	core.RequireNoError(t, coreio.Local.Write(cfg, "{}"))
	got := DetectArchFromConfig(cfg)
	core.AssertEqual(t, "gemma3", got)
}

func TestGguf_ModelTagToGGUFArch_Good(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	got := ModelTagToGGUFArch("gemma-3-1b")
	core.AssertEqual(t, "gemma3", got)
}

func TestGguf_ModelTagToGGUFArch_Bad(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	got := ModelTagToGGUFArch("unknown")
	core.AssertEqual(t, "gemma3", got)
}

func TestGguf_ModelTagToGGUFArch_Ugly(t *core.T) {
	stubName := t.Name()
	core.AssertNotEmpty(t, stubName)
	got := ModelTagToGGUFArch("")
	core.AssertEqual(t, "gemma3", got)
}

func TestGguf_GGUFModelBlobPath_Good(t *core.T) {
	root := t.TempDir()
	writeOllamaManifest(t, root, "gemma")
	got, err := GGUFModelBlobPath(root, "gemma")
	core.RequireNoError(t, err)
	core.AssertContains(t, got, "sha256-abc")
}

func TestGguf_GGUFModelBlobPath_Bad(t *core.T) {
	got, err := GGUFModelBlobPath(t.TempDir(), "missing")
	core.AssertError(t, err)
	core.AssertEqual(t, "", got)
}

func TestGguf_GGUFModelBlobPath_Ugly(t *core.T) {
	root := t.TempDir()
	manifest := core.JoinPath(root, "manifests", "registry.ollama.ai", "library", "gemma", "test")
	core.RequireNoError(t, coreio.Local.EnsureDir(core.PathDir(manifest)))
	core.RequireNoError(t, coreio.Local.Write(manifest, `{"layers":[]}`))
	got, err := GGUFModelBlobPath(root, "gemma:test")
	core.AssertError(t, err)
	core.AssertEqual(t, "", got)
}

func TestGguf_ParseLayerFromTensorName_Good(t *core.T) {
	layer, err := ParseLayerFromTensorName("blk.12.attn_q.weight")
	core.RequireNoError(t, err)
	core.AssertEqual(t, 12, layer)
}

func TestGguf_ParseLayerFromTensorName_Bad(t *core.T) {
	layer, err := ParseLayerFromTensorName("attn_q.weight")
	core.AssertError(t, err)
	core.AssertEqual(t, 0, layer)
}

func TestGguf_ParseLayerFromTensorName_Ugly(t *core.T) {
	layer, err := ParseLayerFromTensorName("prefix.blk.0.value")
	core.RequireNoError(t, err)
	core.AssertEqual(t, 0, layer)
}
