package detect

import (
	"net/url"
	"testing"
)

func TestConfigFilesConsumeTheirSchemasOwner(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"schemas/app.schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema"}`,
		"app/config.json":         `{"$schema": "../schemas/app.schema.json"}`,
		"app/missing.json":        `{"$schema": "../schemas/gone.json"}`,
		"app/tool.json":           `{"$schema": "https://json.schemastore.org/prettierrc"}`,
		"app/package.json":        `{"name": "app", "$schema": "https://example.com/package"}`,
		"app/invoice.yaml":        "$schema: https://schemas.acme.dev/billing/v2/invoice.json\n",
	})
	if c := mustFind(t, cands, "config:schemas/app", "schemas", "app"); c.Confidence != "medium" {
		t.Errorf("an in-repo schema gives confidence %s", c.Confidence)
	}
	mustFind(t, cands, "config:acme/invoice", "acme", "app")
	if len(cands) != 2 {
		t.Errorf("candidates %v", ids(cands))
	}
}

func TestSchemaOwnerIsTheRepoOrTheDomain(t *testing.T) {
	for ref, want := range map[string][2]string{
		"https://github.com/acme/deployer/schemas/settings/v1": {"deployer", "settings"},
		"https://schemas.acme.dev/billing/v2/invoice.json":     {"acme", "invoice"},
		"https://localhost/flags.json":                         {"localhost", "flags"},
		"https://acme.dev/v1":                                  {"", ""},
		"https://json-schema.org/draft/2020-12/schema":         {"", ""},
	} {
		u, _ := url.Parse(ref)
		if owner, name := schemaOwner(u); owner != want[0] || name != want[1] {
			t.Errorf("schemaOwner(%s) = %s, %s", ref, owner, name)
		}
	}
}

func TestATableOnlyItsOwnerUsesIsNoSeam(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"orders/pyproject.toml":            "[project]\nname = \"orders\"\ndependencies = [\"psycopg\"]\n",
		"orders/migrations/0001_init.py":   "op.create_table('orders')\n",
		"orders/app.py":                    "rows = conn.execute(\"SELECT * FROM orders\")\n",
		"billing/requirements.txt":         "# pinned\npsycopg[binary]==3.2\n",
		"billing/pyproject.toml":           "[project]\nname = \"billing\"\n",
		"billing/bill.py":                  "conn.execute(\"SELECT * FROM invoices JOIN customers\")\n",
		"billing/migrations/0001_init.sql": "create table if not exists public.invoices (id int);\ncreate table \"auth\".\"customers\" (id int);\n",
	})
	if len(cands) != 0 {
		t.Errorf("candidates %v: a table read only by its owner isn't a seam, nor is a schema", ids(cands))
	}
}

func TestToolsOwnConfigSchemasArentSeams(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"turbo.json":      `{"$schema": "https://turbo.build/schema.json"}`,
		"web/vercel.json": `{"$schema": "https://openapi.vercel.sh/vercel.json"}`,
	})
	if len(cands) != 0 {
		t.Errorf("candidates %v", ids(cands))
	}
}
