// SPDX-Licence-Identifier: EUPL-1.2

package contracts_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"strconv"

	"dappco.re/go"
)

func TestRootPackageUsesInferencePrimitivesInsteadOfConcreteRuntimes(t *core.T) {
	root := core.JoinPath("..", "..")
	for _, path := range rootPackageGoFiles(t, root) {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imported := range file.Imports {
			value, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("unquote import %s in %s: %v", imported.Path.Value, path, err)
			}
			switch value {
			case "dappco.re/go/mlx", "dappco.re/go/rocm":
				t.Fatalf("%s imports %s; root go-ml package must use dappco.re/go/inference primitives", core.PathBase(path), value)
			}
		}
	}
}

func rootPackageGoFiles(t *core.T, root string) []string {
	t.Helper()
	result := core.ReadDir(core.DirFS(root), ".")
	if !result.OK {
		t.Fatalf("read %s: %v", root, result.Value)
	}
	entries := result.Value.([]fs.DirEntry)
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || !core.HasSuffix(entry.Name(), ".go") {
			continue
		}
		paths = append(paths, core.JoinPath(root, entry.Name()))
	}
	return paths
}
