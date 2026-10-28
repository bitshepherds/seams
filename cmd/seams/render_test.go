package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitshepherds/seams/schemas"
)

// threeSeams are an internal event, a vendor's public API, and a table with
// no single owner.
const threeSeams = `{"id": "event:backend/quote-accepted", "provider": {"name": "backend", "inRepo": true},
		"consumers": [{"name": "billing"}], "audience": "internal",
		"contract": {"location": "asyncapi.yaml", "owner": "backend"}, "deploysIndependently": false,
		"testWorld": [{"side": "consumer", "test": "billing/quote.pact.test.ts", "tier": "gate", "tool": "Pact"}]},
	{"id": "http:stripe/payments", "provider": {"name": "stripe", "external": true},
		"consumers": [{"name": "backend", "inRepo": true}], "audience": "public"},
	{"id": "table:_/quotes", "provider": null,
		"consumers": [{"name": "backend"}, {"name": "reports"}], "audience": "internal"}`

// statusFile writes a status file rating the seams, and returns its path.
func statusFile(t *testing.T, seams string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "status.json")
	doc := `{"schema": "` + schemas.ID("status") + `", "seams": {` + seams + `}}`
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const ratings = `"event:backend/quote-accepted": {"rung": 3, "gaps": ["the provider isn't verified against the contract"]},
	"http:stripe/payments": {"rung": 0, "gaps": ["no contract", "nothing tests it"]}`

func TestShowListsEachSeamWithItsCharacteristicsAndRungGapsFirst(t *testing.T) {
	out := mustSeams(t, "show", "--registry", registryFile(t, threeSeams), "--status", statusFile(t, ratings))
	order := []string{"table:_/quotes\n", "http:stripe/payments\n", "event:backend/quote-accepted\n"}
	at := -1
	for _, id := range order {
		i := strings.Index(out, id)
		if i < at {
			t.Errorf("show didn't list the gaps first:\n%s", out)
		}
		at = i
	}
	for _, want := range []string{
		"rung:                  not rated\n",
		"rung:                  0 of 5\ngaps:                  no contract; nothing tests it\n",
		"kind:                  event\n",
		"provider:              backend (in this repo)\n",
		"boundary:              crosses out of the system\n",
		"contract:              asyncapi.yaml (owned by backend)\n",
		"deploys independently: false\n",
		"consumer test:         billing/quote.pact.test.ts, in the gate, with Pact\n",
		"\n3 seam(s): 2 internal, 1 crossing out of the system\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("show printed\n%s\nwithout %q", out, want)
		}
	}
}

func TestStatusFileSetsEachSeamsRungAndWithoutOneNoRungIsShown(t *testing.T) {
	path := registryFile(t, threeSeams)
	if out := mustSeams(t, "show", "--registry", path); strings.Contains(out, "rung") ||
		!strings.HasPrefix(out, "event:backend/quote-accepted\n") {
		t.Errorf("without a status file, show printed\n%s", out)
	}
	out := mustSeams(t, "show", "--registry", path, "--status", statusFile(t, ratings), "event:backend/quote-accepted")
	if !strings.Contains(out, "rung:                  3 of 5\n") || !strings.Contains(out, "the provider isn't verified") {
		t.Errorf("show of one seam printed\n%s", out)
	}
	if out := mustSeams(t, "show", "--registry", path, "event:backend/quote-accepted"); strings.Contains(out, "rung") {
		t.Errorf("show of one seam, without a status file, printed\n%s", out)
	}
}

func TestStatusFileThatDoesNotMatchItsSchemaIsRefused(t *testing.T) {
	t.Chdir(t.TempDir()) // render's default paths are relative
	path := registryFile(t, threeSeams)
	for _, status := range []string{
		statusFile(t, `"event:backend/quote-accepted": {"rung": 7}`),
		statusFile(t, `"event:backend/quote-accepted": {"rung": "high"}`),
		filepath.Join(t.TempDir(), "missing.json"),
	} {
		for _, args := range [][]string{{"show"}, {"render", "--out", filepath.Join(t.TempDir(), "SEAMS.md")}} {
			if code, _, errs := seams(append(args, "--registry", path, "--status", status)...); code != 1 || !strings.Contains(errs, filepath.Base(status)) {
				t.Errorf("seams %s with the status file %s = %d, %q", args[0], filepath.Base(status), code, errs)
			}
		}
	}
}

func TestShowJSONMatchesItsPublishedSchema(t *testing.T) {
	path, status := registryFile(t, threeSeams), statusFile(t, ratings)
	for _, tc := range []struct {
		schema string
		args   []string
	}{
		{"register", []string{"show", "--json", "--registry", path}},
		{"register", []string{"show", "--json", "--registry", path, "--status", status}},
		{"register", []string{"show", "--json", "--registry", emptyRegistry(t)}},
		{"seam", []string{"show", "--json", "--registry", path, "--status", status, "http:stripe/payments"}},
	} {
		_, out, _ := seams(tc.args...)
		if err := schemas.Validate(tc.schema, []byte(out)); err != nil {
			t.Errorf("seams %s: %v\n%s", strings.Join(tc.args, " "), err, out)
		}
	}
	_, out, _ := seams("show", "--json", "--registry", path, "--status", status, "http:stripe/payments")
	if !strings.Contains(out, `"rung": 0`) || !strings.Contains(out, `"top": 5`) || !strings.Contains(out, `"no contract"`) {
		t.Errorf("show --json printed %s", out)
	}
}

func TestRenderWritesTheRegisterAndADiagramOfComponentsAndSeams(t *testing.T) {
	t.Chdir(t.TempDir()) // render's default paths are relative
	dir := t.TempDir()
	doc := filepath.Join(dir, "docs", "SEAMS.md")
	args := []string{"render", "--registry", registryFile(t, threeSeams), "--out", doc, "--history", filepath.Join(dir, "history.json")}
	if out := mustSeams(t, args...); out != "wrote "+doc+": 3 seam(s)\n" {
		t.Errorf("render printed %q", out)
	}
	data, err := os.ReadFile(doc) //nolint:gosec // the test's own file
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# Seams\n",
		"3 seam(s): 2 internal, 1 crossing out of the system.\n",
		"```mermaid\nflowchart LR\n  subgraph system[\"The system\"]\n    p1[\"backend\"]\n    p2[\"billing\"]\n    p4[\"reports\"]\n    s3{{\"table:_/quotes\"}}\n  end\n  p3[\"stripe\"]\n",
		"  p1 -->|\"event:backend/quote-accepted\"| p2\n  p3 -->|\"http:stripe/payments\"| p1\n  s3 --> p1\n  s3 --> p4\n```\n",
		"| `event:backend/quote-accepted` | event | backend (in this repo) | billing | internal | asyncapi.yaml (owned by backend) | no | consumer: billing/quote.pact.test.ts, in the gate, with Pact |\n",
		"| `table:_/quotes` | table | no single owner | backend, reports | internal | none | not known | — |\n",
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("render wrote\n%s\nwithout %q", data, want)
		}
	}
	if strings.Contains(string(data), "Rung") {
		t.Errorf("without a status file, render wrote a rung:\n%s", data)
	}
	if code, _, errs := seams(append(args, "--check")...); code != 0 {
		t.Errorf("render --check of what it wrote = %d, %q", code, errs)
	}
	if err := os.WriteFile(doc, []byte("# Seams\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := seams(append(args, "--check")...); code != 1 || !strings.Contains(errs, "out of date") {
		t.Errorf("render --check of an edited register = %d, %q", code, errs)
	}
}

func TestHistoryRecordsTheCountAtEachRung(t *testing.T) {
	t.Chdir(t.TempDir()) // render's default paths are relative
	if day := today(); len(day) != len("2026-10-23") {
		t.Errorf("today is %q", day)
	}
	defer func(was func() string) { today = was }(today)
	dir := t.TempDir()
	history, doc := filepath.Join(dir, ".sheepdog", "seams", "history.json"), filepath.Join(dir, "SEAMS.md")
	args := []string{"render", "--registry", registryFile(t, threeSeams), "--out", doc, "--history", history, "--record"}
	today = func() string { return "2026-10-22" }
	mustSeams(t, args...)
	mustSeams(t, append(args, "--status", statusFile(t, ratings))...)
	today = func() string { return "2026-10-23" }
	mustSeams(t, append(args, "--status", statusFile(t, `"http:stripe/payments": {"rung": 2}`))...)
	data, err := os.ReadFile(history) //nolint:gosec // the test's own file
	if err != nil {
		t.Fatal(err)
	}
	if err := schemas.Validate("history", data); err != nil {
		t.Errorf("%v\n%s", err, data)
	}
	got := strings.Join(strings.Fields(string(data)), "")
	for _, want := range []string{
		`{"date":"2026-10-22","seams":3,"rungs":[1,0,0,1,0,0,0],"unrated":1}`,
		`{"date":"2026-10-23","seams":3,"rungs":[0,0,1,0,0,0,0],"unrated":2}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the history is %s, without %s", got, want)
		}
	}
	if strings.Count(got, `"date"`) != 2 {
		t.Errorf("a day recorded twice keeps both: %s", got)
	}
	md, _ := os.ReadFile(doc) //nolint:gosec // the test's own file
	if !strings.Contains(string(md), "| 2026-10-23 | 3 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 2 |\n") {
		t.Errorf("render wrote\n%s", md)
	}
	mustSeams(t, "render", "--registry", registryFile(t, threeSeams), "--out", doc, "--history", history, "--record",
		"--last-status", filepath.Join(dir, "none.json"))
	if md, _ := os.ReadFile(doc); !strings.Contains(string(md), "| 2026-10-23 | 3 | — | — | — | — | — | — | — | — |\n") { //nolint:gosec // the test's own file
		t.Errorf("a count with no status file rendered as\n%s", md)
	}
}

func TestRenderFailsWhereItCannotReadOrWrite(t *testing.T) {
	t.Chdir(t.TempDir()) // render's default paths are relative
	dir := t.TempDir()
	path, file := registryFile(t, threeSeams), filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	badHistory := filepath.Join(dir, "history.json")
	if err := os.WriteFile(badHistory, []byte(`{"counts": []}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--registry", file, "--out", filepath.Join(dir, "a.md")},
		{"--registry", path, "--history", dir},
		{"--registry", path, "--history", badHistory},
		{"--registry", path, "--history", filepath.Join(file, "h.json"), "--record"},
		{"--registry", path, "--history", filepath.Join(dir, "h.json"), "--out", filepath.Join(file, "SEAMS.md")},
		{"--registry", path, "--history", filepath.Join(dir, "h.json"), "--out", dir},
	} {
		if code, _, errs := seams(append([]string{"render"}, args...)...); code != 1 || errs == "" {
			t.Errorf("seams render %s = %d, %q", strings.Join(args, " "), code, errs)
		}
	}
	if code, _, errs := seams("render", "--registry", path, "--history", filepath.Join(dir, "h.json"), "--status", statusFile(t, ratings),
		"--last-status", filepath.Join(file, "status.json")); code != 1 || errs == "" {
		t.Errorf("render keeping its status file under a file = %d, %q", code, errs)
	}
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := seams("render", "--registry", path, "--history", filepath.Join(locked, "sub", "h.json"), "--record"); code != 1 || errs == "" {
		t.Errorf("render --record into a locked directory = %d, %q", code, errs)
	}
	if code, _, _ := seams("render", "--registry", path, "--check", "--record"); code != 2 {
		t.Errorf("render --check --record gave %d", code)
	}
	if code, _, _ := seams("render", "stray"); code != 2 {
		t.Errorf("render with an argument gave %d", code)
	}
	if code, _, _ := seams("show", "--registry", file); code != 1 {
		t.Errorf("show of a broken registry gave %d", code)
	}
}

func TestAContractTestNamesTheToolThatRunsIt(t *testing.T) {
	path := emptyRegistry(t)
	mustSeams(t, "add", "--registry", path, "http:backend/quotes", "--consumer", "web",
		"--test", "consumer:web/quotes.pact.test.ts:gate", "--tool", "Pact")
	if out := mustSeams(t, "show", "--registry", path, "http:backend/quotes"); !strings.Contains(out, "web/quotes.pact.test.ts, in the gate, with Pact\n") {
		t.Errorf("show printed\n%s", out)
	}
}

func TestRenderKeepsItsStatusFileSoCheckDrawsTheSameRegister(t *testing.T) {
	t.Chdir(t.TempDir())
	path := registryFile(t, threeSeams)
	mustSeams(t, "render", "--registry", path, "--status", statusFile(t, ratings))
	if _, err := os.Stat(".sheepdog/seams/status.json"); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := seams("render", "--registry", path, "--check"); code != 0 {
		t.Errorf("render --check, after render --status = %d, %q", code, errs)
	}
	md, _ := os.ReadFile("docs/SEAMS.md")
	if !strings.Contains(string(md), "| Rung | Gaps |") {
		t.Errorf("render wrote\n%s", md)
	}
}

func TestARatingIsFoundByAFormerIDAndOneForNoSeamIsNamed(t *testing.T) {
	path := emptyRegistry(t)
	mustSeams(t, "add", "--registry", path, "http:stripe/payments", "--consumer", "checkout")
	mustSeams(t, "update", "--registry", path, "http:stripe/payments", "--rename", "http:stripe/charges")
	status := statusFile(t, `"http:stripe/payments": {"rung": 2}, "http:ghost/api": {"rung": 5}`)
	code, out, errs := seams("show", "--registry", path, "--status", status)
	if code != 0 || !strings.Contains(out, "rung:                  2 of 6\n") {
		t.Errorf("a rating by a former id: show = %d\n%s", code, out)
	}
	if !strings.Contains(errs, "rates http:ghost/api, which isn't registered") {
		t.Errorf("show warned %q", errs)
	}
	_, out, _ = seams("show", "--json", "--registry", path, "--status", status, "http:stripe/charges")
	if err := schemas.Validate("seam", []byte(out)); err != nil || strings.Contains(out, "gaps") {
		t.Errorf("a rating with no gaps: %v\n%s", err, out)
	}
}

func TestAToolWithNoTestIsRefused(t *testing.T) {
	path := registryFile(t, quotesSeam)
	if code, _, errs := seams("update", "--registry", path, "event:backend/quote-accepted", "--tool", "Pact"); code != 1 || !strings.Contains(errs, "give --test too") {
		t.Errorf("update --tool alone = %d, %q", code, errs)
	}
}
