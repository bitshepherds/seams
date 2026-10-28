package seams

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const (
	modulePath    = "github.com/bitshepherds/seams"
	oldModulePath = "github.com/bitshepherds/sheepdog-seams"
)

// seams is its own tool, not sheepdog's: its module path says so, and nothing
// still imports it under the old one.
func TestModulePathIsSeams(t *testing.T) {
	mod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindSubmatch(mod)
	if m == nil || string(m[1]) != modulePath {
		t.Fatalf("go.mod declares module %q, want %q", mod, modulePath)
	}

	err = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != "." && (strings.HasPrefix(name, ".") || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if p == oldModulePath || strings.HasPrefix(p, oldModulePath+"/") {
				t.Errorf("%s imports %s, under the old module path", path, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
