package detect

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// detectFiles detects the seams of a repo, named repo, holding the files.
func detectFiles(t *testing.T, files map[string]string) []Candidate {
	t.Helper()
	fsys := fstest.MapFS{}
	for p, text := range files {
		fsys[p] = &fstest.MapFile{Data: []byte(strings.TrimPrefix(text, "\n"))}
	}
	res, err := Detect(fsys, "repo")
	if err != nil {
		t.Fatal(err)
	}
	return res.Candidates
}

// mustFind is the candidate with the id, failing the test unless its
// provider and consumers are as given; a provider of "" is none.
func mustFind(t *testing.T, cands []Candidate, id, provider string, consumers ...string) Candidate {
	t.Helper()
	c, ok := find(cands, id)
	if !ok {
		t.Fatalf("no %s in %v", id, ids(cands))
	}
	got := ""
	if c.Provider != nil {
		got = c.Provider.Name
	}
	if consumers == nil {
		consumers = []string{}
	}
	if got != provider || !slices.Equal(names(c.Consumers), consumers) {
		t.Errorf("%s: provider %q, consumers %v; want %q, %v", id, got, names(c.Consumers), provider, consumers)
	}
	return c
}

// signals lists the signals of a candidate's evidence.
func signals(c Candidate) []string {
	var out []string
	for _, e := range c.Evidence {
		if !slices.Contains(out, e.Signal) {
			out = append(out, e.Signal)
		}
	}
	return out
}
