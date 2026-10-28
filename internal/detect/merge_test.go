package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bitshepherds/seams/schemas"
)

func TestAProvidersUnnamedAPIJoinsItsOneNamedAPI(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"catalog-info.yaml":    "kind: Component\nmetadata:\n  name: pricing\nspec:\n  providesApis: [quotes]\n",
		"pricing/package.json": `{"name": "pricing", "dependencies": {"express": "^4.21.0", "@apollo/server": "^4.0.0"}}`,
	})
	mustFind(t, cands, "http:pricing/quotes", "pricing")
	mustFind(t, cands, "graphql:pricing/api", "pricing")
	if len(cands) != 2 {
		t.Errorf("candidates %v", ids(cands))
	}
}

func TestATopicSeveralPartiesPublishHasNoSingleOwner(t *testing.T) {
	publisher := `producer.send({ topic: 'orders' });`
	cands := detectFiles(t, map[string]string{
		"web/package.json":     `{"name": "web", "dependencies": {"kafkajs": "^2.2.4"}}`,
		"web/orders.js":        publisher,
		"admin/package.json":   `{"name": "admin", "dependencies": {"kafkajs": "^2.2.4"}}`,
		"admin/orders.js":      publisher,
		"stock/package.json":   `{"name": "stock", "dependencies": {"kafkajs": "^2.2.4"}}`,
		"stock/orders.js":      `consumer.subscribe({ topic: 'orders' });`,
		"billing/package.json": `{"name": "billing", "dependencies": {"kafkajs": "^2.2.4"}}`,
		"billing/orders.js":    `consumer.subscribe({ topic: 'orders' });`,
	})
	c := mustFind(t, cands, "event:_/orders", "", "billing", "stock")
	if len(c.Notes) != 1 || !strings.Contains(c.Notes[0], "no single owner: admin, web") {
		t.Errorf("notes %q", c.Notes)
	}
}

func TestEvidenceTextIsCut(t *testing.T) {
	long := "app.get('/quotes', " + strings.Repeat("x", 200) + ");\n"
	cands := detectFiles(t, map[string]string{"pricing/server.js": long})
	if text := mustFind(t, cands, "http:pricing/api", "pricing").Evidence[0].Text; len(text) != 120 || !strings.HasSuffix(text, "...") {
		t.Errorf("text %q", text)
	}
}

func TestDetectFailsOnARepoItCannotRead(t *testing.T) {
	if _, err := Detect(os.DirFS(filepath.Join(t.TempDir(), "missing")), "missing"); err == nil {
		t.Error("detected a missing repo")
	}
}

func TestNamesAreMadeSlugs(t *testing.T) {
	for in, want := range map[string]string{"Pricing Service": "pricing-service", "_orders.": "orders"} {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q", in, got)
		}
	}
	if got := kebab("PricingService"); got != "pricing-service" {
		t.Errorf("kebab = %q", got)
	}
}

func TestEverySignalIsPublishedInTheSchema(t *testing.T) {
	src, _ := schemas.Source("detect")
	var doc struct {
		Defs struct {
			Evidence struct {
				Properties struct {
					Signal struct{ Enum []string }
				}
			}
		} `json:"$defs"`
	}
	if err := json.Unmarshal(src, &doc); err != nil {
		t.Fatal(err)
	}
	if got := doc.Defs.Evidence.Properties.Signal.Enum; !slices.Equal(got, Signals) {
		t.Errorf("the schema publishes the signals %v; the code has %v", got, Signals)
	}
}

func TestAServiceTwoComponentsServeKeepsOneCandidatePerId(t *testing.T) {
	grpcMod := "module example.com/%s\n\nrequire google.golang.org/grpc v1.64.0\n"
	cands := detectFiles(t, map[string]string{
		"pricing/go.mod":        strings.ReplaceAll(grpcMod, "%s", "pricing"),
		"pricing/pricing.proto": "service PricingService {}\n",
		"pricing/main.go":       "pb.RegisterPricingServiceServer(s, &pricer{})\n",
		"fake/go.mod":           strings.ReplaceAll(grpcMod, "%s", "fake"),
		"fake/main.go":          "pb.RegisterPricingServiceServer(s, &fake{})\n",
		"checkout/go.mod":       strings.ReplaceAll(grpcMod, "%s", "checkout"),
		"checkout/main.go":      "c := pb.NewPricingServiceClient(conn)\n",
		"reports/package.json":  `{"name": "reports"}`,
		"reports/stats.proto":   "service Stats {}\n",
		"admin/package.json":    `{"name": "admin"}`,
		"admin/stats.proto":     "service Stats {}\n",
		"stats/go.mod":          strings.ReplaceAll(grpcMod, "%s", "stats"),
		"stats/main.go":         "pb.RegisterStatsServer(s, &stats{})\n",
		"stats2/go.mod":         strings.ReplaceAll(grpcMod, "%s", "stats2"),
		"stats2/main.go":        "pb.RegisterStatsServer(s, &stats{})\n",
	})
	// pricing holds the proto, so it's pricing's service checkout calls.
	mustFind(t, cands, "grpc:pricing/pricing-service", "pricing", "checkout")
	mustFind(t, cands, "grpc:fake/pricing-service", "fake")
	// Two components serve Stats, and two hold its proto: nothing says whose it is.
	mustFind(t, cands, "grpc:_/stats", "")
	if !slices.IsSortedFunc(cands, func(a, b Candidate) int { return strings.Compare(a.ID, b.ID) }) {
		t.Errorf("candidates out of order: %v", ids(cands))
	}
}
