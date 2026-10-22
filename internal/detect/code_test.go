package detect

import (
	"slices"
	"testing"
)

func TestDjangoRoutesCountOnlyWhereDjangoIs(t *testing.T) {
	urls := `
from django.urls import path

urlpatterns = [path("quotes/<int:pk>/", views.quote)]
`
	cands := detectFiles(t, map[string]string{
		"pricing/pyproject.toml": "[project]\nname = \"pricing\"\ndependencies = [\"django>=5\"]\n",
		"pricing/urls.py":        urls,
		"tools/pyproject.toml":   "[project]\nname = \"tools\"\n",
		"tools/urls.py":          urls,
	})
	c := mustFind(t, cands, "http:pricing/api", "pricing")
	if !slices.Contains(signals(c), sigHTTPRoute) || len(cands) != 1 {
		t.Errorf("signals %v; candidates %v", signals(c), ids(cands))
	}
}

func TestCallsReachTheOneComponentServingTheirPath(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"pricing/package.json": `{"name": "pricing"}`,
		"pricing/server.js": `
app.get('/quotes/:sku', quote);
app.get('/history/:sku/days', history);
app.get('/health', ok);
`,
		"stock/package.json":    `{"name": "stock"}`,
		"stock/server.js":       "app.get('/health', ok);\n",
		"checkout/package.json": `{"name": "checkout"}`,
		"checkout/calls.js": `
fetch("/quotes?sku=1");
fetch("/health");
fetch("/history/1");
fetch("/quotes/1/items");
fetch("/stock/1");
fetch("http://pricing");
fetch("quotes");
fetch("${base}");
`,
	})
	c := mustFind(t, cands, "http:pricing/api", "pricing", "checkout")
	calls := 0
	for _, e := range c.Evidence {
		if e.Signal == sigRouteCall {
			calls++
		}
	}
	if calls != 1 {
		t.Errorf("want only the call to /quotes matched, got %+v", c.Evidence)
	}
}

func TestBrokerCallsSayWhoSendsAndWhoReceives(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"jobs/package.json": `{"name": "jobs", "dependencies": {"amqplib": "^0.10.0", "nats": "^2.0.0"}}`,
		"jobs/work.js": `
channel.sendToQueue('builds', payload);
nc.publish('alerts', data);
`,
		"runner/package.json": `{"name": "runner", "dependencies": {"amqplib": "^0.10.0", "nats": "^2.0.0"}}`,
		"runner/run.js": `
channel.consume('builds', build);
nc.subscribe('alerts');
`,
		"runner/audit.js": "const topic = 'audit';\n",
	})
	mustFind(t, cands, "queue:runner/builds", "runner", "jobs")
	mustFind(t, cands, "event:jobs/alerts", "jobs", "runner")
	// A topic named with no direction near it is low, raised one by the broker's library.
	if audit := mustFind(t, cands, "event:_/audit", "", "runner"); audit.Confidence != "medium" {
		t.Errorf("a topic with no direction has confidence %s", audit.Confidence)
	}
}

func TestEnvNamesNameTopicsQueuesAndServicesWithoutIaC(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"shop/package.json": `{"name": "shop"}`,
		"shop/index.js": `
const orders = process.env.ORDERS_TOPIC; // publish here
const pay = process.env.STRIPE_URL;
const self = process.env.SHOP_URL;
const db = process.env.DATABASE_URL;
const name = process.env.SHOP_NAME;
`,
	})
	mustFind(t, cands, "event:shop/orders", "shop")
	mustFind(t, cands, "http:stripe/api", "stripe", "shop")
	if len(cands) != 2 {
		t.Errorf("candidates %v: a component's own URL, a database's and other names aren't seams", ids(cands))
	}
}
