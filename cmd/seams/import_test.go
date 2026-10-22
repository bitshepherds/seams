package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func catalogFile(t *testing.T, yaml string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "catalog-info.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const catalogYAML = "kind: Component\nmetadata: {name: backend}\nspec: {providesApis: [quotes]}\n---\n" +
	"kind: Component\nmetadata: {name: web}\nspec: {consumesApis: [quotes]}\n"

func TestImportCatalogInfoAddsTheSeamsItDeclares(t *testing.T) {
	path := emptyRegistry(t)
	out := mustSeams(t, "import", "catalog-info", "--registry", path, "--file", catalogFile(t, catalogYAML))
	if out != "added http:backend/quotes: from catalog-info's API quotes\n" {
		t.Errorf("import printed %q", out)
	}
	if s, ok := mustLoad(t, path).Find("http:backend/quotes"); !ok || s.Consumers[0].Name != "web" {
		t.Fatalf("imported %+v", s)
	}
	if out := mustSeams(t, "import", "catalog-info", "--registry", path, "--file", catalogFile(t, "kind: Location\n")); out != "catalog-info names no APIs\n" {
		t.Errorf("an empty import printed %q", out)
	}
}

func TestImportRefusesWhatItCannotRead(t *testing.T) {
	path := emptyRegistry(t)
	broken := registryFile(t, "")
	if err := os.WriteFile(broken, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	unwritable := filepath.Join(t.TempDir(), "seams.json")
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone", "seams.json"), unwritable); err != nil {
		t.Fatal(err)
	}
	good := catalogFile(t, catalogYAML)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"import", "backstage", "--registry", path}, "not \"backstage\""},
		{[]string{"import", "catalog-info", "--registry", path, "--file", filepath.Join(t.TempDir(), "none.yaml")}, "none.yaml"},
		{[]string{"import", "catalog-info", "--registry", path, "--file", catalogFile(t, "kind: [")}, "catalog-info:"},
		{[]string{"import", "catalog-info", "--registry", broken, "--file", good}, broken},
		{[]string{"import", "catalog-info", "--registry", unwritable, "--file", good}, "seams.json"},
	} {
		if code, _, errs := seams(tc.args...); code != 1 || !strings.Contains(errs, tc.want) {
			t.Errorf("seams %v = %d, %q; want %q", tc.args, code, errs, tc.want)
		}
	}
	if code, _, _ := seams("import", "--registry", path); code != 2 {
		t.Errorf("import with no source gave %d", code)
	}
}
