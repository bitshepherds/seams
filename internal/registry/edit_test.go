package registry

import (
	"strings"
	"testing"
)

func TestAddRefusesATakenIDOrASeamWithProblems(t *testing.T) {
	r := New()
	if err := r.Add(seam("http:backend/quotes", party("backend"), "web")); err != nil {
		t.Fatal(err)
	}
	if err := r.Add(seam("http:backend/quotes", party("backend"), "mobile")); err == nil || !strings.Contains(err.Error(), "registered already") {
		t.Errorf("a taken id: %v", err)
	}
	if err := r.Add(seam("http:backend/orders", party("backend"), "backend")); err == nil || !strings.Contains(err.Error(), "both provider and consumer") {
		t.Errorf("a party on both sides: %v", err)
	}
	if len(r.Seams) != 1 {
		t.Errorf("a refused seam was kept: %+v", r.Seams)
	}
}

func TestUpdateChangesASeamFoundByAFormerIDAndUndoesAChangeWithProblems(t *testing.T) {
	r := New()
	s := seam("http:backend/quotes", party("backend"), "web")
	s.Was = []string{"http:backend/prices"}
	s.Contract = &Contract{Location: "api/openapi.yaml"}
	_ = r.Add(s)
	got, err := r.Update("http:backend/prices", func(s *Seam) { s.Audience = Public })
	if err != nil || got.Audience != Public {
		t.Fatalf("Update = %+v, %v", got, err)
	}
	if _, err := r.Update("http:backend/quotes", func(s *Seam) {
		s.Consumers = append(s.Consumers, Party{Name: "backend"})
		s.Contract.Owner = "someone"
	}); err == nil {
		t.Fatal("a change with a problem was taken")
	}
	if kept, _ := r.Find("http:backend/quotes"); len(kept.Consumers) != 1 || kept.Contract.Owner != "" {
		t.Errorf("the refused change was kept: %+v", kept)
	}
	if _, err := r.Update("http:backend/nothing", func(*Seam) {}); err == nil || !strings.Contains(err.Error(), "no seam") {
		t.Errorf("updating a missing seam: %v", err)
	}
}

func TestRenameKeepsTheOldIDAndMovesTheProviderWithTheID(t *testing.T) {
	s := seam("table:backend/quotes", &Party{Name: "backend", InRepo: true}, "reports")
	Rename(&s, "table:_/quotes")
	if s.Provider != nil || s.Was[0] != "table:backend/quotes" {
		t.Fatalf("to no owner: %+v", s)
	}
	Rename(&s, "table:billing/quotes")
	if s.Provider == nil || s.Provider.Name != "billing" {
		t.Fatalf("to an owner: %+v", s)
	}
	Rename(&s, "table:ledger/quotes")
	if s.Provider.Name != "ledger" || len(s.Was) != 3 {
		t.Fatalf("to another owner: %+v", s)
	}
	Rename(&s, "not an id")
	if rep := registryOf(s).Validate(); rep.OK() {
		t.Fatal("a malformed new id passed validation")
	}
}

func TestRejectedCandidateIsRememberedByItsEvidence(t *testing.T) {
	r := New()
	health := Rejection{ID: "http:backend/health", Signal: "route", File: "backend/main.go", Reason: "a probe"}
	if added, err := r.Reject(health); !added || err != nil {
		t.Fatalf("Reject = %v, %v", added, err)
	}
	if added, _ := r.Reject(health); added {
		t.Error("the same rejection was added twice")
	}
	if !r.Rejects(Rejection{ID: "http:backend/health", Signal: "route", File: "backend/main.go"}) {
		t.Error("the candidate, found again by the same evidence, isn't rejected")
	}
	for _, other := range []Rejection{
		{ID: "http:backend/health", Signal: "env-url", File: "backend/main.go"},
		{ID: "http:backend/health", Signal: "route", File: "backend/server.go"},
		{ID: "http:backend/ready", Signal: "route", File: "backend/main.go"},
	} {
		if r.Rejects(other) {
			t.Errorf("%+v is rejected, but the team rejected only %+v", other, health)
		}
	}
	if _, err := r.Reject(Rejection{ID: "health", Signal: "route", File: "main.go"}); err == nil {
		t.Error("a candidate with a malformed id was rejected")
	}
}
