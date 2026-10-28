package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitshepherds/seams/internal/register"
	"github.com/bitshepherds/seams/internal/registry"
)

// The messages for files from before seams was renamed, word for word.
const (
	oldFolderMessage   = `seams: .sheepdog/seams/ holds seams files from before seams was renamed, which this version doesn't read. Move them with "git mv .sheepdog/seams .seams", then change each file's schema from https://github.com/bitshepherds/sheepdog-seams/schemas/<name>/v1 to https://github.com/bitshepherds/seams/schemas/<name>/v1.` + "\n"
	bothFoldersMessage = `seams: both .sheepdog/seams/ and .seams/ exist. seams reads only .seams/: merge anything you need from .sheepdog/seams/ into it by hand, then delete .sheepdog/seams/.` + "\n"
)

// oldIDMessage is what seams says of a file at path that carries the old
// schema id for the schema name.
func oldIDMessage(path, name string) string {
	return "seams: " + path + " carries the schema id https://github.com/bitshepherds/sheepdog-seams/schemas/" + name +
		"/v1, from before seams was renamed. Change it to https://github.com/bitshepherds/seams/schemas/" + name + "/v1.\n"
}

// mkdirs makes each directory under root.
func mkdirs(t *testing.T, root string, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
}

// oldFile writes a file whose schema is the old id for the schema name.
func oldFile(t *testing.T, name, rest string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name+".json")
	doc := `{"schema": "https://github.com/bitshepherds/sheepdog-seams/schemas/` + name + `/v1", ` + rest + `}`
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaultPathsAreUnderDotSeams(t *testing.T) {
	for got, want := range map[string]string{
		registry.DefaultPath:    ".seams/registry.json",
		register.DefaultStatus:  ".seams/status.json",
		register.DefaultHistory: ".seams/history.json",
	} {
		if got != want {
			t.Errorf("a default path is %q, want %q", got, want)
		}
	}
}

func TestOldSchemaIDIsRefusedWithItsNewID(t *testing.T) {
	t.Chdir(t.TempDir())
	reg := oldFile(t, "registry", `"seams": []`)
	status := oldFile(t, "status", `"seams": {}`)
	history := oldFile(t, "history", `"counts": []`)
	for _, c := range []struct {
		name, path string
		args       []string
	}{
		{"registry", reg, []string{"validate", "--registry", reg}},
		{"status", status, []string{"render", "--registry", registryFile(t, threeSeams), "--status", status}},
		{"history", history, []string{"render", "--registry", registryFile(t, threeSeams), "--history", history}},
	} {
		if code, _, errs := seams(c.args...); code != 1 || errs != oldIDMessage(c.path, c.name) {
			t.Errorf("an old %s schema id gave %d, %q", c.name, code, errs)
		}
	}
}

func TestOldFolderIsRefusedEvenWithAnExplicitRegistry(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	mkdirs(t, dir, ".sheepdog/seams")
	path := emptyRegistry(t)
	for _, args := range [][]string{
		{"add", "--registry", path, "http:backend/quotes", "--consumer", "web"},
		{"detect", "--registry", path},
		{"import", "--registry", path, "catalog-info"},
		{"list", "--registry", path},
		{"reject", "--registry", path, "http:stripe/api", "--signal", "vendor", "--file", "pay.js"},
		{"render", "--registry", path},
		{"render", "--registry", path, "--check"},
		{"show", "--registry", path},
		{"update", "--registry", path, "http:backend/quotes", "--audience", "public"},
		{"validate", "--registry", path},
	} {
		if code, _, errs := seams(args...); code != 1 || errs != oldFolderMessage {
			t.Errorf("seams %s, with the old folder, gave %d, %q", strings.Join(args, " "), code, errs)
		}
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("seams add wrote the registry, with the old folder there")
	}
	mkdirs(t, dir, ".seams")
	if code, _, errs := seams("list", "--registry", path); code != 1 || errs != bothFoldersMessage {
		t.Errorf("with both folders, seams list gave %d, %q", code, errs)
	}
}

func TestDetectChecksTheFolderUnderItsRoot(t *testing.T) {
	t.Chdir(t.TempDir())
	root := t.TempDir()
	mkdirs(t, root, ".sheepdog/seams")
	if code, _, errs := seams("detect", "--root", root, "--registry", emptyRegistry(t)); code != 1 || errs != oldFolderMessage {
		t.Errorf("the old folder under the root gave %d, %q", code, errs)
	}
	mkdirs(t, root, ".seams")
	if code, _, errs := seams("detect", "--root", root); code != 1 || errs != bothFoldersMessage {
		t.Errorf("both folders under the root gave %d, %q", code, errs)
	}
	clean := t.TempDir()
	mkdirs(t, clean, ".seams")
	t.Chdir(root) // the working directory's folders don't count for detect
	if code, _, errs := seams("detect", "--root", clean); code != 0 {
		t.Errorf("a root with only .seams/ gave %d, %q", code, errs)
	}
}

func TestHelpAndVersionIgnoreTheOldFolder(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	mkdirs(t, dir, ".sheepdog/seams")
	if code, out, _ := seams("--version"); code != 0 || !strings.HasPrefix(out, "seams ") {
		t.Errorf("seams --version gave %d, %q", code, out)
	}
	for name := range commands {
		code, _, errs := seams(name, "--help")
		if code != 2 || strings.Contains(errs, "renamed") || !strings.Contains(errs, "Usage of seams "+name) {
			t.Errorf("seams %s --help gave %d, %q", name, code, errs)
		}
	}
}
