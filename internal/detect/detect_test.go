package detect

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bitshepherds/sheepdog-seams/internal/registry"
)

// detectFixture detects the seams of a fixture repo under testdata/.
func detectFixture(t *testing.T, fixture string) *Result {
	t.Helper()
	res, err := Detect(os.DirFS(filepath.Join("..", "..", "testdata", fixture)), filepath.Base(fixture))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func find(cands []Candidate, id string) (Candidate, bool) {
	i := slices.IndexFunc(cands, func(c Candidate) bool { return c.ID == id })
	if i < 0 {
		return Candidate{}, false
	}
	return cands[i], true
}

func names(parties []registry.Party) []string {
	out := []string{}
	for _, p := range parties {
		out = append(out, p.Name)
	}
	return out
}

func ids(cands []Candidate) []string {
	var out []string
	for _, c := range cands {
		out = append(out, c.ID)
	}
	return out
}

// hasEvidence says whether a candidate has evidence from a signal, in a
// file, at a line the file has.
func hasEvidence(t *testing.T, fixture string, c Candidate, signal string) bool {
	t.Helper()
	for _, e := range c.Evidence {
		if e.Signal != signal {
			continue
		}
		data, err := fs.ReadFile(os.DirFS(filepath.Join("..", "..", "testdata", fixture)), e.File)
		if err != nil || e.Line < 1 || e.Line > strings.Count(string(data), "\n")+1 || e.Text == "" {
			t.Errorf("%s: evidence %+v doesn't point at a line of a file", c.ID, e)
		}
		return true
	}
	return false
}

func TestEachSignalKindFindsItsFixtureSeamWithProviderConsumersEvidenceAndConfidence(t *testing.T) {
	for _, tc := range []struct {
		fixture, signal, id, provider string
		consumers                     []string
		confidence                    string
	}{
		{"catalog", sigCatalog, "http:pricing/quotes", "pricing", []string{"checkout"}, "high"},
		{"pact", sigPact, "http:pricing/api", "pricing", []string{"checkout"}, "high"},
		{"contract", sigContract, "http:pricing/api", "pricing", []string{}, "low"},
		{"generator", sigGenerator, "http:pricing/api", "pricing", []string{"checkout"}, "high"},
		{"compat-check", sigCompatCheck, "http:pricing/api", "pricing", []string{}, "high"},
		{"http-server", sigHTTPServer, "http:pricing/api", "pricing", []string{}, "medium"},
		{"http-route", sigHTTPRoute, "http:pricing/api", "pricing", []string{}, "medium"},
		{"http-client", sigHTTPClient, "http:pricing/api", "pricing", []string{"checkout"}, "low"},
		{"env", sigEnv, "http:pricing/api", "pricing", []string{"checkout"}, "medium"},
		{"route-call", sigRouteCall, "http:pricing/api", "pricing", []string{"checkout"}, "high"},
		{"grpc", sigGRPC, "grpc:pricing/pricing", "pricing", []string{"reports"}, "medium"},
		{"org-client", sigOrgClient, "http:pricing/api", "pricing", []string{"checkout"}, "medium"},
		{"workspace", sigWorkspace, "lib:money/api", "money", []string{"checkout"}, "high"},
		{"vendor", sigVendor, "http:stripe/api", "stripe", []string{"checkout"}, "medium"},
		{"messaging", sigMessaging, "event:checkout/orders", "checkout", []string{"fulfilment"}, "high"},
		{"messaging", sigBroker, "event:checkout/orders", "checkout", []string{"fulfilment"}, "high"},
		{"serverless", sigServerless, "queue:mailer/emails", "mailer", []string{}, "high"},
		{"sam", sigServerless, "event:sam/orders", "sam", []string{"fulfilment"}, "high"},
		{"iac-gateway", sigIaCGateway, "http:checkout/api", "checkout", []string{}, "medium"},
		{"iac-resource", sigIaCResource, "event:infra/orders", "infra", []string{}, "low"},
		{"iac-subscription", sigIaCSubscription, "event:infra/orders", "infra", []string{"fulfilment"}, "high"},
		{"remote-state", sigRemoteState, "config:network/state", "network", []string{"infra"}, "medium"},
		{"tables", sigMigrations, "table:fulfilment/shipments", "fulfilment", []string{"reports"}, "high"},
		{"tables", sigDBClient, "table:fulfilment/shipments", "fulfilment", []string{"reports"}, "high"},
		{"config-schema", sigConfigSchema, "config:deployer/settings", "deployer", []string{"app"}, "low"},
	} {
		t.Run(tc.signal+"/"+tc.fixture, func(t *testing.T) {
			fixture := filepath.Join("signals", tc.fixture)
			res := detectFixture(t, fixture)
			if len(res.Candidates) != 1 {
				t.Fatalf("want one candidate, got %v", ids(res.Candidates))
			}
			c := res.Candidates[0]
			switch {
			case c.ID != tc.id:
				t.Errorf("id %s, want %s", c.ID, tc.id)
			case c.Provider == nil || c.Provider.Name != tc.provider:
				t.Errorf("provider %+v, want %s", c.Provider, tc.provider)
			case !slices.Equal(names(c.Consumers), tc.consumers):
				t.Errorf("consumers %v, want %v", names(c.Consumers), tc.consumers)
			case c.Confidence != tc.confidence:
				t.Errorf("confidence %s, want %s", c.Confidence, tc.confidence)
			case !hasEvidence(t, fixture, c, tc.signal):
				t.Errorf("no %s evidence in %+v", tc.signal, c.Evidence)
			}
		})
	}
}

func TestDetectorFindsAtLeast80PercentOfTheFixtureSystemsSeams(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "system.seams.json"))
	if err != nil {
		t.Fatal(err)
	}
	var truth []struct{ ID string }
	if err := json.Unmarshal(data, &truth); err != nil {
		t.Fatal(err)
	}
	cands := detectFixture(t, "system").Candidates
	found := 0
	for _, s := range truth {
		if _, ok := find(cands, s.ID); ok {
			found++
		} else {
			t.Logf("missed %s", s.ID)
		}
	}
	recall := float64(found) / float64(len(truth))
	t.Logf("found %d of %d true seams (%.0f%%), among %d candidates (%.0f%% precise)",
		found, len(truth), 100*recall, len(cands), 100*float64(found)/float64(len(cands)))
	if recall < 0.8 {
		t.Errorf("found %.0f%% of the fixture system's seams, want at least 80%%", 100*recall)
	}
}

func TestVendoredCodeVirtualEnvironmentsAndBuildOutputAreNotSeams(t *testing.T) {
	res := detectFixture(t, "excluded")
	if len(res.Candidates) != 0 {
		t.Errorf("proposed %v from vendored code, virtual environments, build output or tests", ids(res.Candidates))
	}
	if len(res.Components) != 1 || res.Components[0].Name != "shop" {
		t.Errorf("components %+v, want only the root's, shop", res.Components)
	}
}

func TestEnvURLFollowedThroughTerraformFindsTheResourceTwoComponentsShare(t *testing.T) {
	res := detectFixture(t, "env-terraform")
	c, ok := find(res.Candidates, "queue:fulfilment/orders")
	if !ok {
		t.Fatalf("no queue:fulfilment/orders in %v", ids(res.Candidates))
	}
	if !slices.Equal(names(c.Consumers), []string{"checkout"}) || c.Confidence != "high" {
		t.Errorf("consumers %v, confidence %s", names(c.Consumers), c.Confidence)
	}
	// Each component reads its own env name; Terraform sets both to the one queue.
	for _, want := range []struct{ party, file, text string }{
		{"checkout", "checkout/send.go", "ORDERS_URL"},
		{"checkout", "infra/main.tf", "ORDERS_URL = aws_sqs_queue.orders.url"},
		{"fulfilment", "fulfilment/poll.js", "INBOX_URL"},
		{"fulfilment", "infra/main.tf", "INBOX_URL = aws_sqs_queue.orders.url"},
	} {
		if !slices.ContainsFunc(c.Evidence, func(e Evidence) bool {
			return e.Signal == sigEnv && e.Party == want.party && e.File == want.file && strings.Contains(e.Text, want.text)
		}) {
			t.Errorf("no env evidence for %s in %s naming %s: %+v", want.party, want.file, want.text, c.Evidence)
		}
	}
}

func TestThirdPartyCalledFromMoreThanOneModuleSaysSoWithEachSite(t *testing.T) {
	c, ok := find(detectFixture(t, "vendor-spread").Candidates, "http:stripe/api")
	if !ok || len(c.Notes) != 1 {
		t.Fatalf("http:stripe/api: %+v", c)
	}
	for _, want := range []string{"more than one module", "payments/charge/index.js:1", "payments/refund/index.js:1"} {
		if !strings.Contains(c.Notes[0], want) {
			t.Errorf("the note %q doesn't say %q", c.Notes[0], want)
		}
	}
	if !c.Provider.External {
		t.Errorf("stripe isn't external: %+v", c.Provider)
	}
	if one, _ := find(detectFixture(t, "signals/vendor").Candidates, "http:stripe/api"); len(one.Notes) != 0 {
		t.Errorf("a vendor called from one module has notes %q", one.Notes)
	}
}

func TestDriftNamesACandidateTheRegistryLacksAndARegisteredSeamWithNoEvidenceLeft(t *testing.T) {
	cands := detectFixture(t, "signals/messaging").Candidates
	r := registry.New()
	r.Seams = []registry.Seam{
		{ID: "http:legacy/api", Provider: &registry.Party{Name: "legacy"}, Consumers: []registry.Party{{Name: "checkout"}}},
		{ID: "cli:checkout/admin", Provider: &registry.Party{Name: "checkout"}, Consumers: []registry.Party{{Name: "ops"}}},
	}
	d := FindDrift(cands, r)
	if !d.Drifted() || len(d.Missing) != 1 || len(d.Stale) != 1 {
		t.Fatalf("drift %+v", d)
	}
	if m := d.Missing[0]; m.Candidate.ID != "event:checkout/orders" || !strings.Contains(m.Why, "the registry doesn't") {
		t.Errorf("missing %+v", m)
	}
	if s := d.Stale[0]; s.Seam != "http:legacy/api" || !strings.Contains(s.Why, "no") {
		t.Errorf("stale %+v; a cli seam, which nothing detects, is never stale", s)
	}
}

func TestDriftMatchesASeamByIdFormerIdOrWhatTheEvidenceCouldName(t *testing.T) {
	seam := func(id string, was ...string) registry.Seam { return registry.Seam{ID: id, Was: was} }
	for _, tc := range []struct {
		candidate string
		seam      registry.Seam
		want      bool
	}{
		{"event:checkout/orders", seam("event:checkout/orders"), true},
		{"event:checkout/orders", seam("event:checkout/placed", "event:checkout/orders"), true},
		{"http:pricing/api", seam("http:pricing/quotes"), true},
		{"http:pricing/api", seam("http:billing/quotes"), false},
		{"grpc:_/pricing", seam("grpc:pricing/pricing"), true},
		{"event:checkout/orders", seam("queue:checkout/orders"), false},
		{"event:checkout/orders", seam("event:checkout/refunds"), false},
	} {
		if got := matches(Candidate{ID: tc.candidate}, tc.seam); got != tc.want {
			t.Errorf("matches(%s, %s) = %v", tc.candidate, tc.seam.ID, got)
		}
	}
}

func TestRejectedCandidateIsNotProposedAgain(t *testing.T) {
	cands := detectFixture(t, "signals/vendor").Candidates
	r := registry.New()
	r.Rejected = []registry.Rejection{{ID: "http:stripe/api", Signal: sigVendor, File: "checkout/elsewhere.js"}}
	if got := Proposed(cands, r); len(got) != 1 {
		t.Errorf("a rejection of other evidence hid the candidate: %v", ids(got))
	}
	r.Rejected = append(r.Rejected, registry.Rejection{ID: "http:stripe/api", Signal: sigVendor, File: "checkout/pay.js"})
	if got := Proposed(cands, r); len(got) != 0 {
		t.Errorf("proposed the rejected %v", ids(got))
	}
	if d := FindDrift(Proposed(cands, r), r); d.Drifted() {
		t.Errorf("a rejected candidate is drift: %+v", d)
	}
}
