package registry

import (
	"strings"
	"testing"
)

const catalog = `apiVersion: backstage.io/v1alpha1
kind: Component
metadata:
  name: backend
spec:
  providesApis: [quotes, api:default/quote-events, orphan]
---
apiVersion: backstage.io/v1alpha1
kind: Component
metadata:
  name: web
spec:
  consumesApis: [quotes, prices, quote-events]
---
apiVersion: backstage.io/v1alpha1
kind: Component
metadata:
  name: Mailer
spec:
  consumesApis: [quote-events]
---
apiVersion: backstage.io/v1alpha1
kind: API
metadata:
  name: quote-events
spec:
  type: asyncapi
---
apiVersion: backstage.io/v1alpha1
kind: API
metadata:
  name: prices
spec:
  type: some-format
`

func changesByAction(changes []Change) map[string][]string {
	by := map[string][]string{}
	for _, c := range changes {
		by[c.Action] = append(by[c.Action], c.Seam)
	}
	return by
}

func TestCatalogInfoProvidesAPIsAndConsumesAPIsFillInProviderAndConsumers(t *testing.T) {
	r := registryOf(seam("http:backend/quotes", party("backend"), "mobile"))
	changes, err := r.ImportCatalog([]byte(catalog))
	if err != nil {
		t.Fatal(err)
	}
	by := changesByAction(changes)
	if strings.Join(by["filled"], ",") != "http:backend/quotes" || strings.Join(by["added"], ",") != "event:backend/quote-events" {
		t.Fatalf("changes: %+v", changes)
	}
	quotes, _ := r.Find("http:backend/quotes")
	if len(quotes.Consumers) != 2 || quotes.Consumers[1].Name != "web" {
		t.Errorf("quotes: %+v", quotes)
	}
	events, _ := r.Find("event:backend/quote-events")
	if events.Provider.Name != "backend" || len(events.Consumers) != 2 || events.Consumers[1].Name != "mailer" {
		t.Errorf("quote-events: %+v", events)
	}
	// prices has no provider, and orphan no consumer, in the catalog.
	if strings.Join(by["skipped"], ",") != "orphan,prices" {
		t.Errorf("skipped: %v", by["skipped"])
	}
	if rep := r.Validate(); !rep.OK() {
		t.Errorf("problems after the import: %+v", rep.Problems)
	}
}

func TestCatalogInfoThatDisagreesWithTheRegistryLeavesTheRegistryAsItWas(t *testing.T) {
	r := registryOf(
		seam("http:billing/quotes", party("billing"), "mobile"),
		seam("event:_/quote-events", nil, "backend", "mailer", "web"),
		seam("http:prices/prices", party("prices"), "web"),
	)
	// Already invalid, with its provider as a consumer, so no addition is taken.
	r.Seams[2].Consumers = []Party{{Name: "prices"}}
	changes, err := r.ImportCatalog([]byte(catalog))
	if err != nil {
		t.Fatal(err)
	}
	by := changesByAction(changes)
	if strings.Join(by["disagrees"], ",") != "http:prices/prices,event:_/quote-events,http:billing/quotes" {
		t.Fatalf("changes: %+v", changes)
	}
	if quotes, _ := r.Find("http:billing/quotes"); quotes.Provider.Name != "billing" {
		t.Errorf("the registry's provider wasn't kept: %+v", quotes)
	}
	if prices, _ := r.Find("http:prices/prices"); len(prices.Consumers) != 1 {
		t.Errorf("a consumer was added to a seam with problems: %+v", prices)
	}
}

func TestCatalogInfoThatIsNotYAMLIsRefused(t *testing.T) {
	if _, err := New().ImportCatalog([]byte("kind: [")); err == nil || !strings.Contains(err.Error(), "catalog-info") {
		t.Fatalf("ImportCatalog = %v", err)
	}
}

func TestCatalogInfoAPIThatCannotBeASeamIsSkipped(t *testing.T) {
	cat := "kind: Component\nmetadata: {name: Back End}\nspec: {providesApis: [quotes]}\n---\n" +
		"kind: Component\nmetadata: {name: web}\nspec: {consumesApis: [quotes]}\n"
	changes, err := New().ImportCatalog([]byte(cat))
	if err != nil || len(changes) != 1 || changes[0].Action != "skipped" {
		t.Fatalf("ImportCatalog = %+v, %v", changes, err)
	}
}
