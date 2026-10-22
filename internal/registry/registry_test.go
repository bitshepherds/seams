package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func party(name string) *Party { return &Party{Name: name} }

func seam(id string, provider *Party, consumers ...string) Seam {
	s := Seam{ID: id, Provider: provider, Audience: Internal}
	for _, c := range consumers {
		s.Consumers = append(s.Consumers, Party{Name: c})
	}
	return s
}

func registryOf(seams ...Seam) *Registry {
	r := New()
	r.Seams = seams
	return r
}

// problemAbout is the one problem the report has about the seam, failing
// the test unless there is exactly one.
func problemAbout(t *testing.T, rep Report, seam string) string {
	t.Helper()
	if len(rep.Problems) != 1 || rep.Problems[0].Seam != seam {
		t.Fatalf("want one problem about %s, got %+v", seam, rep.Problems)
	}
	return rep.Problems[0].Message
}

func TestRegistryRefusesAnIDThatIsNotKindProviderNameNamingTheSeam(t *testing.T) {
	for id, want := range map[string]string{
		"quote-accepted":             "kind:provider/name",
		"event:quote-accepted":       "kind:provider/name",
		"rpc:backend/quotes":         `kind "rpc"`,
		"event:Backend/quote":        `provider "Backend"`,
		"event:backend/Quote":        `name "Quote"`,
		"event:backend/quote/second": "kind:provider/name",
	} {
		msg := problemAbout(t, registryOf(seam(id, party("backend"), "billing")).Validate(), id)
		if !strings.Contains(msg, want) || !strings.Contains(msg, IDForm) {
			t.Errorf("%s: %q doesn't say %q and %s", id, msg, want, IDForm)
		}
	}
}

func TestRegistryRefusesAnIDThatDoesNotNameTheSeamsProvider(t *testing.T) {
	msg := problemAbout(t, registryOf(seam("http:backend/quotes", party("frontend"), "app")).Validate(), "http:backend/quotes")
	if !strings.Contains(msg, "provided by frontend") {
		t.Errorf("got %q", msg)
	}
	msg = problemAbout(t, registryOf(seam("table:backend/quotes", nil, "app")).Validate(), "table:backend/quotes")
	if !strings.Contains(msg, "names _ as provider") {
		t.Errorf("got %q", msg)
	}
}

func TestRegistryRefusesAPartyNameThatIsNotLowerCase(t *testing.T) {
	msg := problemAbout(t, registryOf(seam("http:backend/quotes", party("backend"), "Billing")).Validate(), "http:backend/quotes")
	if !strings.Contains(msg, `"Billing"`) {
		t.Errorf("got %q", msg)
	}
}

func TestRenamedSeamKeepsItsOldIDUnderWasAndIsFoundByEither(t *testing.T) {
	s := seam("event:backend/quote-accepted", party("backend"), "billing")
	s.Was = []string{"event:backend/quote-agreed"}
	r := registryOf(s)
	if rep := r.Validate(); !rep.OK() {
		t.Fatalf("problems: %+v", rep.Problems)
	}
	for _, id := range []string{"event:backend/quote-accepted", "event:backend/quote-agreed"} {
		if got, ok := r.Find(id); !ok || got.ID != "event:backend/quote-accepted" {
			t.Errorf("Find(%s) = %v, %v", id, got, ok)
		}
	}
	if _, ok := r.Find("event:backend/nothing"); ok {
		t.Error("found a seam that isn't there")
	}
}

func TestRegistryRefusesAnIDTwoSeamsShare(t *testing.T) {
	old := seam("event:backend/quote-accepted", party("backend"), "billing")
	old.Was = []string{"event:backend/quote-agreed"}
	reused := seam("event:backend/quote-agreed", party("backend"), "mailer")
	msg := problemAbout(t, registryOf(old, reused).Validate(), "event:backend/quote-agreed")
	if !strings.Contains(msg, "is event:backend/quote-accepted's too") {
		t.Errorf("got %q", msg)
	}
	bad := seam("event:backend/quote-accepted", party("backend"), "billing")
	bad.Was = []string{"not an id"}
	msg = problemAbout(t, registryOf(bad).Validate(), "event:backend/quote-accepted")
	if !strings.Contains(msg, "former id") {
		t.Errorf("got %q", msg)
	}
}

func TestTwoRegistriesThatNameTheSameSeamGiveItTheSameID(t *testing.T) {
	// Two repos each register the event, each naming only its own consumer.
	backend := registryOf(seam("event:backend/quote-accepted", party("backend"), "billing"))
	mailer := registryOf(seam("event:backend/quote-accepted", party("backend"), "mailer"))
	for _, r := range []*Registry{backend, mailer} {
		if rep := r.Validate(); !rep.OK() {
			t.Fatalf("problems: %+v", rep.Problems)
		}
	}
	a, _ := backend.Find("event:backend/quote-accepted")
	b, _ := mailer.Find("event:backend/quote-accepted")
	if a == nil || b == nil || a.ID != b.ID {
		t.Fatalf("the registries name the seam differently: %v, %v", a, b)
	}
}

func TestSeamNamesOneProviderAndItsConsumersAndAComponentInBothIsRefused(t *testing.T) {
	ok := registryOf(seam("http:backend/quotes", party("backend"), "web", "mobile"))
	if rep := ok.Validate(); !rep.OK() {
		t.Fatalf("problems: %+v", rep.Problems)
	}
	both := registryOf(seam("http:backend/quotes", party("backend"), "web", "backend"))
	msg := problemAbout(t, both.Validate(), "http:backend/quotes")
	if !strings.Contains(msg, "backend as both provider and consumer") {
		t.Errorf("got %q", msg)
	}
}

func TestSeamWithNoSingleOwnerIsRecordedWithNoProviderAndReported(t *testing.T) {
	r := registryOf(seam("table:_/quotes", nil, "backend", "reports"))
	rep := r.Validate()
	if !rep.OK() {
		t.Fatalf("problems: %+v", rep.Problems)
	}
	if len(rep.Findings) != 1 || rep.Findings[0].Seam != "table:_/quotes" || !strings.Contains(rep.Findings[0].Message, "no single owner") {
		t.Fatalf("findings: %+v", rep.Findings)
	}
}

func TestLocalSDKOverAThirdPartyIsTwoSeamsWithTheSDKAConsumerInTheSecond(t *testing.T) {
	sdk := seam("lib:acme-client/orders", party("acme-client"), "checkout", "admin")
	vendor := seam("http:acme/orders", &Party{Name: "acme", External: true}, "acme-client")
	r := registryOf(sdk, vendor)
	if rep := r.Validate(); !rep.OK() {
		t.Fatalf("problems: %+v", rep.Problems)
	}
	want := []Role{{Seam: "lib:acme-client/orders", Provides: true}, {Seam: "http:acme/orders"}}
	if got := r.Roles("acme-client"); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Roles(acme-client) = %+v", got)
	}
}

func TestRejectedCandidateWithAMalformedIDIsRefused(t *testing.T) {
	r := New()
	r.Rejected = []Rejection{{ID: "quotes", Signal: "env-url", File: "web/.env"}}
	if msg := problemAbout(t, r.Validate(), "quotes"); !strings.Contains(msg, "rejected candidate") {
		t.Errorf("got %q", msg)
	}
}

func TestRegistrySavesAndLoadsAsItWas(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".sheepdog", "seams.json")
	independent := true
	s := seam("http:stripe/payments", &Party{Name: "stripe", External: true}, "checkout")
	s.Consumers[0].InRepo = true
	s.Audience = Public
	s.Contract = &Contract{Location: "https://docs.stripe.com/api", Owner: "stripe"}
	s.DeploysIndependently = &independent
	s.TestWorld = []ContractTest{{Side: "consumer", Test: "checkout/stripe_test.go", Tier: "gate"}}
	r := registryOf(s)
	r.Rejected = []Rejection{{ID: "http:backend/health", Signal: "route", File: "backend/main.go", Reason: "a probe"}}
	if err := r.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	g := got.Seams[0]
	if g.ID != s.ID || !g.Provider.External || !g.Consumers[0].InRepo || g.Contract.Owner != "stripe" ||
		!*g.DeploysIndependently || g.TestWorld[0].Tier != "gate" || got.Rejected[0].Reason != "a probe" {
		t.Fatalf("loaded %+v", got)
	}
}

func TestLoadingAMissingRegistryGivesAnEmptyOne(t *testing.T) {
	r, err := Load(filepath.Join(t.TempDir(), "seams.json"))
	if err != nil || len(r.Seams) != 0 || r.Schema == "" {
		t.Fatalf("Load = %+v, %v", r, err)
	}
}

func TestLoadingARegistryThatDoesNotMatchItsSchemaSaysWhere(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seams.json")
	if err := os.WriteFile(path, []byte(`{"seams": [{"id": "http:backend/quotes"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("Load = %v", err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("loaded a directory")
	}
}

func TestSavingWhereNoDirectoryCanBeMadeFails(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := New().Save(filepath.Join(file, "seams.json")); err == nil {
		t.Fatal("saved under a file")
	}
}
