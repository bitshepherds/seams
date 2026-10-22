package detect

import (
	"fmt"
	"slices"

	"github.com/bitshepherds/sheepdog-seams/internal/registry"
)

// Drift is how the registry differs from the code.
type Drift struct {
	Missing []Missing `json:"missing"`
	Stale   []Stale   `json:"stale"`
}

// Missing is a candidate the registry lacks.
type Missing struct {
	Candidate Candidate `json:"candidate"`
	Why       string    `json:"why"`
}

// Stale is a registered seam the code no longer has evidence for.
type Stale struct {
	Seam string `json:"seam"`
	Why  string `json:"why"`
}

// Drifted says whether there's any drift.
func (d Drift) Drifted() bool { return len(d.Missing)+len(d.Stale) > 0 }

// detectable are the kinds of seam detection has signals for: a registered
// seam of another kind, such as a CLI, is never stale.
var detectable = []string{"http", "grpc", "graphql", "event", "queue", "table", "config", "lib"}

// FindDrift compares the candidates, less those rejected, with the
// registry's seams.
func FindDrift(cands []Candidate, r *registry.Registry) Drift {
	d := Drift{Missing: []Missing{}, Stale: []Stale{}}
	for _, c := range cands {
		if !slices.ContainsFunc(r.Seams, func(s registry.Seam) bool { return matches(c, s) }) {
			e := c.Evidence[slices.IndexFunc(c.Evidence, func(e Evidence) bool { return e.Role != supports })]
			d.Missing = append(d.Missing, Missing{c, fmt.Sprintf(
				"the code has it, by %s at %s:%d, but the registry doesn't", e.Signal, e.File, e.Line)})
		}
	}
	for _, s := range r.Seams {
		id, _ := registry.ParseID(s.ID) // a loaded registry's ids parse
		if slices.Contains(detectable, id.Kind) && !slices.ContainsFunc(cands, func(c Candidate) bool { return matches(c, s) }) {
			d.Stale = append(d.Stale, Stale{s.ID, "registered, but nothing in the code is evidence of it any more"})
		}
	}
	return d
}

// matches says whether a candidate is a registered seam: by its id, or one
// the seam had; or, where the evidence couldn't name a part of it, by the
// rest. A provider's unnamed API matches its one registered API of the
// kind, and a seam of unknown provider matches by kind and name.
func matches(c Candidate, s registry.Seam) bool {
	if c.ID == s.ID || slices.Contains(s.Was, c.ID) {
		return true
	}
	cid, _ := registry.ParseID(c.ID)
	sid, _ := registry.ParseID(s.ID)
	switch {
	case cid.Kind != sid.Kind:
		return false
	case cid.Name == generic:
		return cid.Provider == sid.Provider
	case cid.Provider == registry.NoProvider:
		return cid.Name == sid.Name
	}
	return false
}
