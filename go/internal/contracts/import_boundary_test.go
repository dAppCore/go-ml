// SPDX-Licence-Identifier: EUPL-1.2

package contracts_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRootPackageUsesInferencePrimitivesInsteadOfConcreteRuntimes(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
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
				t.Fatalf("%s imports %s; root go-ml package must use dappco.re/go/inference primitives", filepath.Base(path), value)
			}
		}
	}
}

func rootPackageGoFiles(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		paths = append(paths, filepath.Join(root, entry.Name()))
	}
	return paths
}
