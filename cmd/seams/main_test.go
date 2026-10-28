package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitshepherds/seams/schemas"
)

// seams runs the command line, returning its exit code, stdout and stderr.
func seams(args ...string) (int, string, string) {
	var out, errs strings.Builder
	code := run(args, &out, &errs)
	return code, out.String(), errs.String()
}

// registryFile writes a registry to a temporary file, and returns its path.
func registryFile(t *testing.T, seams string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "seams.json")
	doc := `{"schema": "` + schemas.ID("registry") + `", "seams": [` + seams + `]}`
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const (
	quotesSeam = `{"id": "event:backend/quote-accepted", "provider": {"name": "backend"},
		"consumers": [{"name": "billing"}], "audience": "internal"}`
	ownerlessSeam = `{"id": "table:_/quotes", "provider": null,
		"consumers": [{"name": "backend"}, {"name": "reports"}], "audience": "internal"}`
	malformedSeam = `{"id": "quote-accepted", "provider": {"name": "backend"},
		"consumers": [{"name": "billing"}], "audience": "internal"}`
)

func TestVersionPrintsTheStampedVersion(t *testing.T) {
	defer func(v string) { version = v }(version)
	version = "1.2.3"
	if code, out, _ := seams("--version"); code != 0 || out != "seams 1.2.3\n" {
		t.Fatalf("seams --version = %d, %q", code, out)
	}
}

func TestSeamsRefusesAnUnknownCommandAndListsTheCommands(t *testing.T) {
	for _, args := range [][]string{{"nope"}, {}} {
		code, _, errs := seams(args...)
		if code != 2 || !strings.Contains(errs, "usage") || !strings.Contains(errs, "validate") {
			t.Errorf("seams %v = %d, %q", args, code, errs)
		}
	}
}

func TestACommandRefusesUnknownFlagsAndStrayArguments(t *testing.T) {
	if code, _, _ := seams("validate", "--nope"); code != 2 {
		t.Errorf("an unknown flag gave %d", code)
	}
	if code, _, errs := seams("validate", "stray"); code != 2 || !strings.Contains(errs, "want 0 argument(s)") {
		t.Errorf("a stray argument gave %d, %q", code, errs)
	}
}

func TestValidatePassesAGoodRegistryAndReportsFindings(t *testing.T) {
	code, out, _ := seams("validate", "--registry", registryFile(t, quotesSeam+","+ownerlessSeam))
	if code != 0 || !strings.Contains(out, "finding: table:_/quotes has no single owner") || !strings.Contains(out, "2 seam(s), 0 problem(s), 1 finding(s)") {
		t.Fatalf("validate = %d, %q", code, out)
	}
}

func TestValidateFailsARegistryWithProblemsNamingTheSeam(t *testing.T) {
	code, out, errs := seams("validate", "--registry", registryFile(t, malformedSeam))
	if code != 1 || !strings.Contains(out, "problem: quote-accepted") || !strings.Contains(out, "kind:provider/name") || !strings.Contains(errs, "problems") {
		t.Fatalf("validate = %d, %q, %q", code, out, errs)
	}
}

func TestValidateFailsARegistryItCannotRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seams.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := seams("validate", "--registry", path); code != 1 || !strings.Contains(errs, path) {
		t.Fatalf("validate = %d, %q", code, errs)
	}
}

func TestEachCommandsJSONOutputMatchesItsPublishedSchema(t *testing.T) {
	for _, tc := range []struct {
		schema string
		args   []string
	}{
		{"validate", []string{"validate", "--json", "--registry", registryFile(t, quotesSeam+","+ownerlessSeam)}},
		{"validate", []string{"validate", "--json", "--registry", registryFile(t, malformedSeam)}},
		{"list", []string{"list", "--json", "--registry", registryFile(t, quotesSeam+","+ownerlessSeam)}},
		{"list", []string{"list", "--json", "--registry", emptyRegistry(t)}},
		{"seam", []string{"show", "--json", "--registry", registryFile(t, quotesSeam), "event:backend/quote-accepted"}},
		{"seam", []string{"add", "--json", "--registry", emptyRegistry(t), "http:backend/quotes", "--consumer", "web"}},
		{"seam", []string{"update", "--json", "--registry", registryFile(t, quotesSeam), "event:backend/quote-accepted", "--audience", "public"}},
		{"import", []string{"import", "catalog-info", "--json", "--registry", emptyRegistry(t), "--file", catalogFile(t, catalogYAML)}},
		{"import", []string{"import", "catalog-info", "--json", "--registry", emptyRegistry(t), "--file", catalogFile(t, "kind: Location\n")}},
		{"rejection", []string{"reject", "--json", "--registry", emptyRegistry(t), "http:backend/health", "--signal", "route", "--file", "main.go"}},
	} {
		_, out, _ := seams(tc.args...)
		if err := schemas.Validate(tc.schema, []byte(out)); err != nil {
			t.Errorf("seams %s: %v\n%s", strings.Join(tc.args, " "), err, out)
		}
	}
}
