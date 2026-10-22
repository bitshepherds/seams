package detect

import (
	"slices"
	"testing"
)

func TestProtoServicesAndBufGenerationMakeGRPCSeams(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"protos/pricing.proto": "syntax = \"proto3\";\n\nservice PricingService {\n  rpc Quote (Sku) returns (Price);\n}\n",
		"protos/buf.gen.yaml":  "version: v2\n",
		"pricing/go.mod":       "module example.com/pricing\n\nrequire google.golang.org/grpc v1.64.0\n",
		"pricing/main.go":      "pb.RegisterPricingServiceServer(s, &pricer{})\n",
	})
	c := mustFind(t, cands, "grpc:pricing/pricing-service", "pricing")
	if c.Contract != "protos/pricing.proto" || !slices.Contains(signals(c), sigContract) {
		t.Errorf("contract %q, signals %v", c.Contract, signals(c))
	}
}

func TestGraphQLAndAsyncAPIContractsDeclareTheirAPIs(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"catalog/schema.graphql": "\"\"\"The catalogue.\"\"\"\ntype Query {\n  items: [Item]\n}\n",
		"catalog/notes.graphql":  "type Item {\n  id: ID\n}\n",
		"events/asyncapi.yaml":   "asyncapi: 3.0.0\nchannels:\n  orders.placed:\n    address: orders.placed\n",
		"events/broken.yaml":     "asyncapi: 3.0.0\nchannels: [\n",
	})
	if c := mustFind(t, cands, "graphql:catalog/api", "catalog"); c.Contract != "catalog/schema.graphql" {
		t.Errorf("contract %q", c.Contract)
	}
	mustFind(t, cands, "event:events/orders.placed", "events")
	if len(cands) != 2 {
		t.Errorf("candidates %v", ids(cands))
	}
}

func TestCatalogAPIsOfUnknownTypeAreHTTPAndPactsTheirEvents(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"catalog-info.yaml":  "kind: Component\nmetadata:\n  name: billing\nspec:\n  consumesApis:\n    - \"inv\\x6Fices\"\n",
		"pacts/events.json":  `{"consumer": {"name": "billing"}, "provider": {"name": "orders"}, "messages": [{}]}`,
		"pacts/unnamed.json": `{"consumer": {"name": ""}, "provider": {"name": "orders"}}`,
	})
	c := mustFind(t, cands, "http:_/invoices", "", "billing")
	if c.Confidence != "medium" || c.Evidence[0].Line != 1 || c.Notes[0] != "the evidence doesn't say who provides it" {
		t.Errorf("%+v", c)
	}
	mustFind(t, cands, "event:orders/api", "orders", "billing")
}

func TestGeneratorsAndCompatibilityChecksNameTheAPIsTheyUse(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"web/package.json":        `{"name": "web"}`,
		"web/openapitools.json":   `{"generator-cli": {"generators": {"pricing": {"inputSpec": "http://pricing:8080/openapi.json"}}}}`,
		"web/kiota-lock.json":     `{"descriptionLocation": "../missing/openapi.yaml"}`,
		".gitlab-ci.yml":          "verify:\n  script:\n    - pact-broker can-i-deploy --pacticipant web --latest\n",
		".github/workflows/x.yml": "jobs: {}\n",
	})
	c := mustFind(t, cands, "http:pricing/api", "pricing", "web")
	if !slices.Contains(signals(c), sigCompatCheck) || !slices.Contains(signals(c), sigGenerator) {
		t.Errorf("signals %v: can-i-deploy supports web's seams", signals(c))
	}
}
