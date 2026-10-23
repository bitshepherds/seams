// Package register draws a repo's register of seams from its registry: each
// seam with its characteristics and test world and, where a status file
// rates it, its rung on the coverage ladder. Another tool writes the status
// file, so seams never judges practices itself.
package register

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bitshepherds/sheepdog-seams/internal/registry"
	"github.com/bitshepherds/sheepdog-seams/schemas"
)

// Top is the coverage ladder's top rung. Rung 0 is no coverage at all.
const Top = 6

// DefaultStatus is where seams render keeps the status file it last used, so
// --check draws the same register.
const DefaultStatus = ".sheepdog/seams-status.json"

// Status is each seam's rung and gaps, as another tool judged them.
type Status struct {
	Schema string            `json:"schema"`
	Seams  map[string]Rating `json:"seams"`
}

// Rating is one seam's rung on the coverage ladder, and what it lacks to climb.
type Rating struct {
	Rung int      `json:"rung"`
	Gaps []string `json:"gaps,omitempty"`
}

// LoadStatus reads a status file, refusing one that doesn't match its schema.
func LoadStatus(path string) (*Status, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the status file's path is the user's to choose
	if err != nil {
		return nil, err
	}
	if err := schemas.Validate("status", data); err != nil {
		return nil, fmt.Errorf("%s doesn't match the status schema: %w", path, err)
	}
	var s Status
	_ = json.Unmarshal(data, &s) // the schema has already passed it
	return &s, nil
}

// Entry is one seam in the register.
type Entry struct {
	Seam registry.Seam `json:"seam"`
	Kind string        `json:"kind"`
	// CrossesOut is true when any party is external, or the audience is public.
	CrossesOut bool `json:"crossesOut"`
	// Rung is nil when no status file rates the seam.
	Rung *int     `json:"rung,omitempty"`
	Gaps []string `json:"gaps,omitempty"`
}

// Rate is the registry's seam's entry, rated by the status file when there
// is one, by its id or one it had before a rename.
func Rate(r *registry.Registry, s registry.Seam, st *Status) Entry {
	kind, _, _ := strings.Cut(s.ID, ":")
	e := Entry{Seam: s, Kind: kind, CrossesOut: crossesOut(s, externals(r))}
	if st == nil {
		return e
	}
	for _, id := range append([]string{s.ID}, s.Was...) {
		if rating, ok := st.Seams[id]; ok {
			e.Rung, e.Gaps = &rating.Rung, rating.Gaps
			return e
		}
	}
	return e
}

// externals are the parties marked external in any seam: a party is outside
// the system in all of them, or in none.
func externals(r *registry.Registry) map[string]bool {
	out := map[string]bool{}
	for _, s := range r.Seams {
		for _, p := range sides(s) {
			out[p.Name] = out[p.Name] || p.External
		}
	}
	return out
}

func crossesOut(s registry.Seam, external map[string]bool) bool {
	if s.Audience == registry.Public {
		return true
	}
	for _, p := range sides(s) {
		if external[p.Name] {
			return true
		}
	}
	return false
}

// Unknown lists the ids the status file rates that name no registered seam,
// so a rating isn't lost without a word.
func (st *Status) Unknown(r *registry.Registry) []string {
	var ids []string
	for id := range st.Seams {
		if _, ok := r.Find(id); !ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// Save writes the status file to path, making its directory if need be.
func (st *Status) Save(path string) error {
	data, _ := json.MarshalIndent(st, "", "  ") // a Status always marshals
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// Build is the register: every seam, in the registry's order or, with a
// status file, gaps first: those it doesn't rate, then the lowest rung up.
func Build(r *registry.Registry, st *Status) []Entry {
	entries := make([]Entry, len(r.Seams))
	for i, s := range r.Seams {
		entries[i] = Rate(r, s, st)
	}
	if st != nil {
		sort.SliceStable(entries, func(i, j int) bool { return rank(entries[i]) < rank(entries[j]) })
	}
	return entries
}

// rank orders an entry, gaps first: an unrated seam before rung 0.
func rank(e Entry) int {
	if e.Rung == nil {
		return -1
	}
	return *e.Rung
}
