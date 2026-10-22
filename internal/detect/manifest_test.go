package detect

import "testing"

func TestPoetryAndRequirementsDependenciesAreRead(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"api/pyproject.toml":   "[tool.poetry]\nname = \"api\"\n\n[tool.poetry.dependencies]\npython = \"^3.12\"\nflask = \"^3.0\"\n",
		"web/pyproject.toml":   "[project]\nname = \"web\"\n",
		"web/requirements.txt": "stripe==10.0\n",
	})
	mustFind(t, cands, "http:api/api", "api")
	mustFind(t, cands, "http:stripe/api", "stripe", "web")
}

func TestTheOrgsOwnClientIsItsServicesSeam(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"checkout/go.mod":       "module github.com/acme/checkout\n\nrequire (\n\tgithub.com/acme/pricing-sdk v1.2.0\n\tgithub.com/other/billing-client v1.0.0\n)\n",
		"web/package.json":      `{"name": "@acme/web", "dependencies": {"@acme/stock-client": "^1.0.0", "@aws-sdk/client-s3": "^3.0.0"}}`,
		"site/package.json":     `{"name": "site", "dependencies": {"prom-client": "^15.0.0", "aws-sdk": "^2.0.0"}}`,
		"worker/pyproject.toml": "[project]\nname = \"worker\"\ndependencies = [\"prometheus-client\", \"opentelemetry-sdk\"]\n",
	})
	mustFind(t, cands, "http:pricing/api", "pricing", "checkout")
	mustFind(t, cands, "http:stock/api", "stock", "web")
	if len(cands) != 2 {
		t.Errorf("candidates %v: another org's client, or one with no org to share, isn't the org's own", ids(cands))
	}
}

func TestTestsAndGeneratedCodeAreNotRead(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"pricing/server_test.go": `mux.HandleFunc("/quotes", quote)`,
		"pricing/pricing.pb.go":  `mux.HandleFunc("/quotes", quote)`,
	})
	if len(cands) != 0 {
		t.Errorf("candidates %v", ids(cands))
	}
}

func TestAGoModuleIsReadWhereverItsModuleLineIs(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"pricing/go.mod": "// The pricing service.\nmodule example.com/pricing\n\nrequire github.com/go-chi/chi/v5 v5.0.12\n",
	})
	mustFind(t, cands, "http:pricing/api", "pricing")
}
