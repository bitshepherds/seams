package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitshepherds/seams/schemas"
)

// fixture is a fixture repo's path, under testdata/.
func fixture(name string) string { return filepath.Join("..", "..", "testdata", name) }

func TestDetectListsEachCandidateWithItsEvidence(t *testing.T) {
	out := mustSeams(t, "detect", "--root", fixture("signals/messaging"), "--registry", emptyRegistry(t))
	for _, want := range []string{
		"event:checkout/orders  checkout → fulfilment  (high, internal)\n",
		"    provides checkout/publish.js:6  messaging  await producer.send(",
		"1 candidate(s) in 2 component(s)\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("seams detect printed %q, without %q", out, want)
		}
	}
	out = mustSeams(t, "detect", "--root", fixture("signals/remote-state"), "--registry", emptyRegistry(t))
	if !strings.Contains(out, "network → infra") || !strings.Contains(out, "note: a seam to another repo") {
		t.Errorf("seams detect printed %q", out)
	}
	out = mustSeams(t, "detect", "--root", fixture("signals/contract"), "--registry", emptyRegistry(t))
	if !strings.Contains(out, "pricing → (none seen)") {
		t.Errorf("seams detect printed %q", out)
	}
	out = mustSeams(t, "detect", "--root", fixture("signals/catalog"), "--registry", emptyRegistry(t))
	if !strings.Contains(out, "http:pricing/quotes  pricing → checkout") {
		t.Errorf("seams detect printed %q", out)
	}
}

func TestDetectNamesASeamWithNoKnownProvider(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "catalog-info.yaml"), []byte("kind: Component\nmetadata:\n  name: web\nspec:\n  consumesApis: [quotes]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out := mustSeams(t, "detect", "--root", root); !strings.Contains(out, "http:_/quotes  (no single owner) → web") {
		t.Errorf("seams detect printed %q", out)
	}
}

func TestDetectJSONOutputMatchesItsPublishedSchema(t *testing.T) {
	for _, tc := range []struct {
		schema string
		args   []string
	}{
		{"detect", []string{"detect", "--root", fixture("system"), "--json"}},
		{"drift", []string{"detect", "--root", fixture("system"), "--drift", "--json"}},
	} {
		_, out, _ := seams(append(tc.args, "--registry", registryFile(t, quotesSeam))...)
		if err := schemas.Validate(tc.schema, []byte(out)); err != nil {
			t.Errorf("seams %s: %v\n%s", strings.Join(tc.args, " "), err, out)
		}
	}
}

func TestDetectDriftExitsOneNamingWhatDrifted(t *testing.T) {
	path := registryFile(t, `{"id": "http:legacy/api", "provider": {"name": "legacy"}, "consumers": [{"name": "checkout"}], "audience": "internal"}`)
	code, out, errs := seams("detect", "--drift", "--root", fixture("signals/messaging"), "--registry", path)
	if code != 1 || !strings.Contains(errs, "drifted") {
		t.Errorf("seams detect --drift = %d, %q", code, errs)
	}
	for _, want := range []string{
		"missing: event:checkout/orders  the code has it, by messaging at checkout/publish.js:6, but the registry doesn't\n",
		"stale: http:legacy/api  registered, but nothing in the code is evidence of it any more\n",
		"1 missing, 1 stale\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("seams detect --drift printed %q, without %q", out, want)
		}
	}
	seam := `{"id": "event:checkout/orders", "provider": {"name": "checkout"}, "consumers": [{"name": "fulfilment"}], "audience": "internal"}`
	if out := mustSeams(t, "detect", "--drift", "--root", fixture("signals/messaging"), "--registry", registryFile(t, seam)); out != "0 missing, 0 stale\n" {
		t.Errorf("with no drift, seams detect --drift printed %q", out)
	}
}

func TestDetectReadsTheRegistryInTheRootItReads(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".sheepdog", "seams"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".sheepdog", "seams", "registry.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := seams("detect", "--root", root); code != 1 || !strings.Contains(errs, "registry.json") {
		t.Errorf("a broken registry in the root gave %d, %q", code, errs)
	}
	if code, _, errs := seams("detect", "--root", filepath.Join(root, "missing"), "--registry", emptyRegistry(t)); code != 1 || errs == "" {
		t.Errorf("a missing root gave %d, %q", code, errs)
	}
	if code, _, _ := seams("detect", "stray"); code != 2 {
		t.Errorf("a stray argument gave %d", code)
	}
}

func TestARejectedCandidateIsNotDetectedAgain(t *testing.T) {
	path := emptyRegistry(t)
	mustSeams(t, "reject", "--registry", path, "http:stripe/api", "--signal", "vendor", "--file", "checkout/pay.js", "--reason", "a test double")
	if out := mustSeams(t, "detect", "--root", fixture("signals/vendor"), "--registry", path); out != "0 candidate(s) in 1 component(s)\n" {
		t.Errorf("seams detect printed %q", out)
	}
}
