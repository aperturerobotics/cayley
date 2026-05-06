package cayley

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var knownProductionReflectImports = []string{
	"internal/linkedql/schema/schema.go",
	"query/gizmo/gizmo.go",
	"query/linkedql/registry.go",
	"schema/loader.go",
	"schema/namespaces.go",
	"schema/schema.go",
	"schema/writer.go",
}

func TestKnownProductionReflectImports(t *testing.T) {
	var got []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch path {
			case ".git", "vendor":
				return filepath.SkipDir
			case "graph/graphtest", "graph/kv/kvtest", "query/path/pathtest":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		importsReflect, err := fileImportsReflect(path)
		if err != nil {
			return err
		}
		if importsReflect {
			got = append(got, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	slices.Sort(got)
	if !slices.Equal(got, knownProductionReflectImports) {
		t.Fatalf("production reflect imports changed\nwant: %v\n got: %v", knownProductionReflectImports, got)
	}
}

func TestDynamicPackagesExcludedFromTinyGo(t *testing.T) {
	for _, dir := range []string{
		"internal/linkedql/schema",
		"query/gizmo",
		"query/linkedql",
		"schema",
	} {
		t.Run(dir, func(t *testing.T) {
			err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}

				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if !strings.Contains(string(b), "//go:build !tinygo") {
					t.Fatalf("%s is missing !tinygo build tag", path)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func fileImportsReflect(path string) (bool, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		return false, err
	}
	for _, imp := range f.Imports {
		if imp.Path.Value == `"reflect"` {
			return true, nil
		}
	}
	return false, nil
}
