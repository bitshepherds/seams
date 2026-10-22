package main

import (
	"strings"
	"testing"
)

func TestListShowsEachSeamOnALine(t *testing.T) {
	path := emptyRegistry(t)
	mustSeams(t, "add", "--registry", path, "event:backend/quote-accepted", "--consumer", "billing")
	mustSeams(t, "add", "--registry", path, "table:_/quotes", "--consumer", "backend,reports", "--in-repo", "reports")
	want := "event:backend/quote-accepted  backend → billing (internal)\n" +
		"table:_/quotes  no single owner → backend, reports (in this repo) (internal)\n"
	if out := mustSeams(t, "list", "--registry", path); out != want {
		t.Errorf("list printed\n%s", out)
	}
	if code, _, _ := seams("list", "--registry", path, "stray"); code != 2 {
		t.Errorf("list with an argument gave %d", code)
	}
}

func TestShowPrintsOneSeamFoundByItsIDOrAFormerOne(t *testing.T) {
	path := emptyRegistry(t)
	mustSeams(t, "add", "--registry", path, "http:stripe/payments", "--consumer", "checkout", "--external", "stripe",
		"--audience", "public", "--contract", "https://docs.stripe.com/api", "--owner", "stripe", "--deploys-independently", "true")
	mustSeams(t, "update", "--registry", path, "http:stripe/payments", "--rename", "http:stripe/charges")
	out := mustSeams(t, "show", "--registry", path, "http:stripe/payments")
	for _, want := range []string{
		"http:stripe/charges\n", "was:", "http:stripe/payments", "stripe (external)", "checkout",
		"public", "https://docs.stripe.com/api (owned by stripe)", "deploys independently: true",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("show printed\n%s\nwithout %q", out, want)
		}
	}
	if code, _, errs := seams("show", "--registry", path, "http:stripe/nothing"); code != 1 || !strings.Contains(errs, "no seam") {
		t.Errorf("show of a missing seam = %d, %q", code, errs)
	}
	if code, _, _ := seams("show", "--registry", path); code != 2 {
		t.Errorf("show with no id gave %d", code)
	}
}

func TestShowPrintsAContractWithNoOwnerAndTheTestWorld(t *testing.T) {
	path := registryFile(t, `{"id": "http:backend/quotes", "provider": {"name": "backend"}, "consumers": [{"name": "web"}],
		"audience": "internal", "contract": {"location": "api/openapi.yaml"},
		"testWorld": [{"side": "consumer", "test": "web/quotes.pact.test.ts", "tier": "gate"}]}`)
	out := mustSeams(t, "show", "--registry", path, "http:backend/quotes")
	if !strings.Contains(out, "api/openapi.yaml\n") || !strings.Contains(out, "consumer test:") || !strings.Contains(out, "web/quotes.pact.test.ts, in the gate") {
		t.Errorf("show printed\n%s", out)
	}
}
