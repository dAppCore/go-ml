package ml

import "strings"

// AgentConfig holds scoring agent configuration.
type AgentConfig struct {
	M3Host        string
	M3User        string
	M3SSHKey      string
	M3AdapterBase string
	InfluxURL     string
	InfluxDB      string
	DBPath        string
	APIURL        string
	JudgeURL      string
	JudgeModel    string
	Model         string
	BaseModel     string
	PollInterval  int
	WorkDir       string
	Filter        string
	Force         bool
	OneShot       bool
	DryRun        bool

	// Transport is the remote transport used for SSH commands and file transfers.
	// If nil, an SSHTransport is created from M3Host/M3User/M3SSHKey.
	Transport RemoteTransport
}

// transport returns the configured RemoteTransport, lazily creating an
// SSHTransport from the M3 fields if none was set.
func (c *AgentConfig) transport() RemoteTransport {
	if c.Transport != nil {
		return c.Transport
	}
	c.Transport = NewSSHTransport(c.M3Host, c.M3User, c.M3SSHKey)
	return c.Transport
}

// Checkpoint represents a discovered adapter checkpoint on M3.
type Checkpoint struct {
	RemoteDir string
	Filename  string
	Dirname   string
	Iteration int
	ModelTag  string
	Label     string
	RunID     string
}

// BaseModelMap maps model tags to their HuggingFace/local model paths.
var BaseModelMap = map[string]string{
	"gemma-3-1b":  "mlx-community/gemma-3-1b-it-4bit",
	"gemma-3-4b":  "mlx-community/gemma-3-4b-it-4bit",
	"gemma-3-12b": "mlx-community/gemma-3-12b-it-4bit",
	"gemma-3-27b": "mlx-community/gemma-3-27b-it-qat-4bit",
	"gpt-oss-20b": "/Volumes/Data/lem/models/gpt-oss-20b-mlx",
}

// ModelFamilies identifies known model families from adapter directory names.
var ModelFamilies = []struct {
	DirPrefix string
	Tag       string
	Short     string
}{
	{"deepseek-r1-7b", "deepseek-r1-7b", "R1"},
	{"27b-", "gemma-3-27b", "G27"},
	{"27b", "gemma-3-27b", "G27"},
	{"15k/gemma-3-27b", "gemma-3-27b", "G27"},
	{"15k/gemma-3-12b", "gemma-3-12b", "G12"},
	{"15k/gemma-3-1b", "gemma-3-1b", "G1"},
	{"12b", "gemma-3-12b", "G12"},
	{"1b-", "gemma-3-1b", "G1"},
	{"1b", "gemma-3-1b", "G1"},
	{"4b", "gemma-3-4b", "G4"},
	{"vi-12b", "gemma-3-12b", "Vi12"},
	{"vi", "gemma-3-1b", "Vi1"},
	{"gpt-oss", "gpt-oss-20b", "GPT"},
	{"lem-gpt-oss", "gpt-oss-20b", "LGPT"},
	{"bench-1b", "gemma-3-1b", "B1"},
	{"book", "gemma-3-27b", "Book"},
	{"cross", "gemma-3-12b", "Cross"},
}

// AdapterMeta maps an adapter directory name to (model_tag, label_prefix, run_id_stem).
func AdapterMeta(dirname string) (string, string, string) {
	name := strings.TrimPrefix(dirname, "adapters-")

	for _, fam := range ModelFamilies {
		if strings.HasPrefix(name, fam.DirPrefix) {
			variant := strings.TrimPrefix(name, fam.DirPrefix)
			variant = strings.TrimLeft(variant, "-")
			if variant == "" {
				variant = "base"
			}
			short := fam.Short + "-" + variant
			if variant == "base" {
				short = fam.Short
			}
			stem := strings.ReplaceAll(name, "/", "-")
			return fam.Tag, short, stem
		}
	}

	short := name
	if len(short) > 10 {
		short = short[:10]
	}
	return name, short, name
}
