package registry

import "fmt"

// Report is what validating a registry found. A problem makes the registry
// invalid; a finding is a fact about the system the team should know, such as
// a seam with no single owner.
type Report struct {
	Problems []Note `json:"problems"`
	Findings []Note `json:"findings"`
}

// Note is one problem or finding, about one seam.
type Note struct {
	Seam    string `json:"seam"`
	Message string `json:"message"`
}

// OK says whether the registry has no problems.
func (rep Report) OK() bool { return len(rep.Problems) == 0 }

// Validate checks what the schema can't: each id's form, that it names the
// seam's provider, that no party is both provider and consumer of one seam,
// and that no two seams share an id.
func (r *Registry) Validate() Report {
	rep := Report{Problems: []Note{}, Findings: []Note{}}
	problem := func(seam, format string, a ...any) {
		rep.Problems = append(rep.Problems, Note{seam, fmt.Sprintf(format, a...)})
	}
	seen := map[string]string{} // id or former id → the seam that holds it
	claim := func(id, seam string) {
		if other, ok := seen[id]; ok {
			problem(seam, "the id %s is %s's too", id, other)
			return
		}
		seen[id] = seam
	}
	for _, s := range r.Seams {
		for _, msg := range checkSeam(s) {
			problem(s.ID, "%s", msg)
		}
		if s.Provider == nil {
			rep.Findings = append(rep.Findings, Note{s.ID, "has no single owner: nobody owns the exchange surface, so nobody can change it safely"})
		}
		claim(s.ID, s.ID)
		for _, was := range s.Was {
			if _, err := ParseID(was); err != nil {
				problem(s.ID, "its former id %v", err)
			}
			claim(was, s.ID)
		}
	}
	for _, rj := range r.Rejected {
		if _, err := ParseID(rj.ID); err != nil {
			problem(rj.ID, "the rejected candidate %v", err)
		}
	}
	return rep
}

// checkSeam lists what's wrong with one seam on its own.
func checkSeam(s Seam) []string {
	var msgs []string
	id, err := ParseID(s.ID)
	if err != nil {
		return []string{err.Error() + "; a seam's id is " + IDForm}
	}
	switch {
	case s.Provider == nil && id.Provider != NoProvider:
		msgs = append(msgs, fmt.Sprintf("has no provider, so its id names %s as provider, not %s", NoProvider, id.Provider))
	case s.Provider != nil && s.Provider.Name != id.Provider:
		msgs = append(msgs, fmt.Sprintf("is provided by %s, but its id names %s", s.Provider.Name, id.Provider))
	}
	parties := s.Consumers
	if s.Provider != nil {
		parties = append([]Party{*s.Provider}, parties...)
	}
	for _, p := range parties {
		if !ValidName(p.Name) {
			msgs = append(msgs, fmt.Sprintf("names the party %q, which isn't lower-case letters, digits, '.', '_' or '-'", p.Name))
		}
	}
	for _, c := range s.Consumers {
		if s.Provider != nil && c.Name == s.Provider.Name {
			msgs = append(msgs, fmt.Sprintf("has %s as both provider and consumer; a party is one or the other in a seam", c.Name))
		}
	}
	return msgs
}
