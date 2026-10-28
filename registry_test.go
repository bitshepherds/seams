package seams

import (
	"testing"

	"github.com/bitshepherds/seams/internal/registry"
)

// This repo practises what it checks: its CLI's output is a seam, in its own
// registry, with sheepdog as a consumer.
func TestThisReposRegistryListsCLISeamsRegistryWithSheepdogAsAConsumer(t *testing.T) {
	r, err := registry.Load(registry.DefaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if rep := r.Validate(); !rep.OK() {
		t.Fatalf("problems: %+v", rep.Problems)
	}
	s, ok := r.Find("cli:seams/registry")
	if !ok {
		t.Fatal("cli:seams/registry isn't registered")
	}
	if s.Provider.Name != "seams" || s.Audience != registry.Public || len(s.Consumers) != 1 || s.Consumers[0].Name != "sheepdog" {
		t.Fatalf("cli:seams/registry is %+v", s)
	}
}
