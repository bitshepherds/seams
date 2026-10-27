package register

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitshepherds/sheepdog-seams/internal/registry"
	"github.com/bitshepherds/sheepdog-seams/schemas"
)

func seam(id, provider string, audience string, consumers ...registry.Party) registry.Seam {
	s := registry.Seam{ID: id, Consumers: consumers, Audience: audience}
	if provider != "" {
		s.Provider = &registry.Party{Name: provider}
	}
	return s
}

func TestRegisterSeparatesInternalSeamsFromThoseThatCrossOutOfTheSystem(t *testing.T) {
	vendor := seam("http:stripe/payments", "stripe", registry.Internal, registry.Party{Name: "backend"})
	vendor.Provider.External = true
	r := &registry.Registry{Seams: []registry.Seam{
		seam("event:backend/quotes", "backend", registry.Internal, registry.Party{Name: "billing"}),
		vendor,
		seam("http:backend/api", "backend", registry.Public),
		seam("file:backend/export", "backend", registry.Internal, registry.Party{Name: "bank", External: true}),
	}}
	entries := Build(r, nil)
	for i, crosses := range []bool{false, true, true, true} {
		if entries[i].CrossesOut != crosses {
			t.Errorf("%s crosses out: %v", entries[i].Seam.ID, entries[i].CrossesOut)
		}
	}
	doc := Markdown(entries, nil, false)
	internal, out := strings.Index(doc, "## Internal seams"), strings.Index(doc, "## Seams that cross out of the system")
	if i := strings.Index(doc, "`event:backend/quotes`"); i < internal || i > out {
		t.Errorf("the internal seam isn't under its heading:\n%s", doc)
	}
	for _, id := range []string{"`http:stripe/payments`", "`http:backend/api`", "`file:backend/export`"} {
		if strings.Index(doc, id) < out {
			t.Errorf("%s isn't under the seams that cross out:\n%s", id, doc)
		}
	}
	if !strings.Contains(doc, "  end\n  p3[\"stripe\"]\n  p4[\"bank\"]\n") {
		t.Errorf("the external parties aren't outside the system:\n%s", doc)
	}
}

func TestAnEmptyRegisterSaysSo(t *testing.T) {
	doc := Markdown(Build(&registry.Registry{}, &Status{}), nil, true)
	want := "# Seams\n\n<!-- Written by seams render, from the registry. Run it again rather than editing this file. -->\n\n" +
		"0 seam(s): 0 internal, 0 crossing out of the system.\n\n## Internal seams\n\nNone.\n\n## Seams that cross out of the system\n\nNone.\n"
	if doc != want {
		t.Errorf("an empty register is\n%s", doc)
	}
}

func TestTheDiagramAndTablesEscapeWhatWouldBreakThem(t *testing.T) {
	s := seam("http:backend/quotes", "backend", registry.Public, registry.Party{Name: `web "beta"`})
	s.Contract = &registry.Contract{Location: "a|b.yaml"}
	s.TestWorld = []registry.ContractTest{{Side: "consumer", Test: "web\nquotes", Tier: "gate"}}
	yes := true
	s.DeploysIndependently = &yes
	public := seam("table:_/rates", "", registry.Public, registry.Party{Name: "backend"})
	doc := Markdown([]Entry{Rate(&registry.Registry{}, s, nil), Rate(&registry.Registry{}, public, nil)}, nil, false)
	for _, want := range []string{`p2["web #quot;beta#quot;"]`, `a\|b.yaml`, "consumer: web quotes, in the gate |", "| yes |", "  s2{{\"table:_/rates\"}}\n"} {
		if !strings.Contains(doc, want) {
			t.Errorf("render wrote\n%s\nwithout %q", doc, want)
		}
	}
}

func TestTheDiagramDrawsTheSystemOnlyWhenAPartyIsInsideIt(t *testing.T) {
	vendor := seam("http:stripe/payments", "stripe", registry.Public)
	vendor.Provider.External = true
	if doc := Markdown([]Entry{Rate(&registry.Registry{}, vendor, nil)}, nil, false); strings.Contains(doc, "subgraph") {
		t.Errorf("a system with no party inside drew a subgraph:\n%s", doc)
	}
	shared := seam("table:_/quotes", "", registry.Internal, registry.Party{Name: "backend"})
	if doc := Markdown([]Entry{Rate(&registry.Registry{}, shared, nil)}, nil, false); !strings.Contains(doc, "    s1{{\"table:_/quotes\"}}\n  end\n  s1 --> p1\n") {
		t.Errorf("a shared internal seam is drawn as\n%s", doc)
	}
}

func TestARatedRegisterListsGapsFirstAndCountsEachRung(t *testing.T) {
	no := false
	event := seam("event:backend/quotes", "backend", registry.Internal, registry.Party{Name: "billing"})
	event.Provider.InRepo = registry.Yes()
	event.Contract = &registry.Contract{Location: "asyncapi.yaml", Owner: "backend"}
	event.DeploysIndependently = &no
	event.TestWorld = []registry.ContractTest{
		{Side: "consumer", Test: "quote.pact.test.ts", Tier: "gate", Tool: "Pact"},
		{Side: "provider", Test: "TestQuotes", Tier: "ci"},
	}
	r := &registry.Registry{Seams: []registry.Seam{
		event,
		seam("http:backend/api", "backend", registry.Public),
		seam("table:_/quotes", "", registry.Internal, registry.Party{Name: "backend", External: true, InRepo: registry.Yes()}),
	}}
	st := &Status{Seams: map[string]Rating{
		"event:backend/quotes": {Rung: 4, Gaps: []string{"the provider isn't verified"}},
		"http:backend/api":     {Rung: 1, Gaps: []string{"no contract", "no compatibility check"}},
	}}
	entries := Build(r, st)
	var ids []string
	for _, e := range entries {
		ids = append(ids, e.Seam.ID)
	}
	if strings.Join(ids, " ") != "table:_/quotes http:backend/api event:backend/quotes" {
		t.Errorf("the register's order is %v", ids)
	}
	h := &History{}
	h.Record(CountOf(entries, "2026-10-22", false))
	h.Record(CountOf(entries, "2026-10-23", false))
	h.Record(CountOf(entries, "2026-10-23", true))
	if len(h.Counts) != 2 || *h.Counts[1].Unrated != 1 || h.Counts[1].Rungs[1] != 1 || h.Counts[1].Rungs[4] != 1 {
		t.Errorf("the history is %+v", h.Counts)
	}
	doc := Markdown(entries, h, true)
	for _, want := range []string{
		"| Seam | Kind | Provider | Consumers | Audience | Contract | Deploys independently | Test world | Rung | Gaps |\n",
		"| `event:backend/quotes` | event | backend (in this repo) | billing | internal | asyncapi.yaml (owned by backend) | no | " +
			"consumer: quote.pact.test.ts, in the gate, with Pact; provider: TestQuotes, in the ci | 4 of 6 | the provider isn't verified |\n",
		"| `table:_/quotes` | table | no single owner | backend (external, in this repo) | internal | none | not known | — | not rated | — |\n",
		"| `http:backend/api` | http | backend | ",
		"| 1 of 6 | no contract; no compatibility check |\n",
		"## History\n\n| Date | Seams | Rung 0 | Rung 1 | Rung 2 | Rung 3 | Rung 4 | Rung 5 | Rung 6 | Not rated |\n",
		"| 2026-10-22 | 3 | — | — | — | — | — | — | — | — |\n| 2026-10-23 | 3 | 0 | 1 | 0 | 0 | 1 | 0 | 0 | 1 |\n",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("render wrote\n%s\nwithout %q", doc, want)
		}
	}
}

func TestStatusAndHistoryFilesAreReadAndWritten(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadStatus(dir); err == nil {
		t.Error("read a directory as a status file")
	}
	path := filepath.Join(dir, "status.json")
	if err := os.WriteFile(path, []byte(`{"schema": "`+schemas.ID("status")+`", "seams": {"http:a/b": {"rung": 4}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"seams": {}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadStatus(path); err == nil || !strings.Contains(err.Error(), "status schema") {
		t.Errorf("a status file that doesn't match its schema: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"schema": "`+schemas.ID("status")+`", "seams": {"http:a/b": {"rung": 4}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := LoadStatus(path)
	if err != nil || st.Seams["http:a/b"].Rung != 4 {
		t.Errorf("LoadStatus = %v, %v", st, err)
	}
	h, err := LoadHistory(filepath.Join(dir, "none.json"))
	if err != nil || len(h.Counts) != 0 || h.Schema != schemas.ID("history") {
		t.Errorf("a missing history = %v, %v", h, err)
	}
	h.Record(Count{Date: "2026-10-23", Seams: 1})
	saved := filepath.Join(dir, "sub", "history.json")
	if err := h.Save(saved); err != nil {
		t.Fatal(err)
	}
	if again, err := LoadHistory(saved); err != nil || len(again.Counts) != 1 {
		t.Errorf("the saved history = %v, %v", again, err)
	}
	if err := h.Save(filepath.Join(path, "history.json")); err == nil {
		t.Error("saved a history under a file")
	}
	if _, err := LoadHistory(dir); err == nil {
		t.Error("read a directory as a history")
	}
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadHistory(path); err == nil || !strings.Contains(err.Error(), "history schema") {
		t.Errorf("a history that doesn't match its schema: %v", err)
	}
}

func TestAPartyExternalInOneSeamIsOutsideTheSystemInAll(t *testing.T) {
	vendor := seam("http:stripe/payments", "stripe", registry.Internal, registry.Party{Name: "checkout"})
	vendor.Provider.External = true
	r := &registry.Registry{Seams: []registry.Seam{vendor,
		seam("http:checkout/orders", "checkout", registry.Internal, registry.Party{Name: "stripe"})}}
	for _, e := range Build(r, nil) {
		if !e.CrossesOut {
			t.Errorf("%s doesn't cross out of the system", e.Seam.ID)
		}
	}
}

func TestAPartyNotInThisRepoIsDrawnLikeOneNotRecorded(t *testing.T) {
	tool := seam("cli:tool/registry", "tool", registry.Internal, registry.Party{Name: "app", InRepo: registry.Yes()})
	tool.Provider.InRepo = registry.No()
	unrecorded := tool
	unrecorded.Provider = &registry.Party{Name: "tool"}
	if got, want := Markdown(Build(&registry.Registry{Seams: []registry.Seam{tool}}, nil), nil, false),
		Markdown(Build(&registry.Registry{Seams: []registry.Seam{unrecorded}}, nil), nil, false); got != want {
		t.Errorf("a provider not in this repo is drawn as\n%s\nnot as\n%s", got, want)
	}
	if got := ProviderName(tool); got != "tool" {
		t.Errorf("a provider not in this repo is named %q", got)
	}
}

func TestStatusNamesTheRatingsForNoSeam(t *testing.T) {
	r := &registry.Registry{Seams: []registry.Seam{seam("http:a/b", "a", registry.Internal)}}
	st := &Status{Seams: map[string]Rating{"http:a/b": {Rung: 1}, "http:z/y": {}, "http:c/d": {}}}
	if got := strings.Join(st.Unknown(r), " "); got != "http:c/d http:z/y" {
		t.Errorf("Unknown = %s", got)
	}
	dir := t.TempDir()
	if err := st.Save(filepath.Join(dir, "sub", "status.json")); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(filepath.Join(dir, "sub", "status.json", "x")); err == nil {
		t.Error("saved a status file under a file")
	}
}
