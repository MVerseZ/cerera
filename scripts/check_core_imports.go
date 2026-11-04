// Command check_core_imports verifies core packages do not import forbidden internal packages.
package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

var forbidden = []string{
	"github.com/cerera/internal/service",
	"github.com/cerera/internal/coinbase",
	"github.com/cerera/internal/observer",
}

func main() {
	root := filepath.Join("core")
	fset := token.NewFileSet()
	var failed bool

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				if p == bad {
					fmt.Fprintf(os.Stderr, "%s: forbidden import %s\n", path, p)
					failed = true
				}
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk: %v\n", err)
		os.Exit(1)
	}
	if failed {
		os.Exit(1)
	}
}
