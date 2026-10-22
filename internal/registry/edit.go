package registry

import (
	"fmt"
	"strings"
)

// Add registers a new seam, refusing it if its id is taken, or if it would
// leave the registry with a problem about it.
func (r *Registry) Add(s Seam) error {
	if _, ok := r.Find(s.ID); ok {
		return fmt.Errorf("%s is registered already", s.ID)
	}
	r.Seams = append(r.Seams, s)
	if err := r.problemsWith(s.ID); err != nil {
		r.Seams = r.Seams[:len(r.Seams)-1]
		return err
	}
	return nil
}

// Update changes the registered seam with the id, or a former one, through
// change, refusing the change if it would leave a problem about the seam.
func (r *Registry) Update(id string, change func(*Seam)) (*Seam, error) {
	s, ok := r.Find(id)
	if !ok {
		return nil, fmt.Errorf("no seam %s", id)
	}
	before := cloneSeam(*s)
	change(s)
	if err := r.problemsWith(s.ID); err != nil {
		*s = before
		return nil, err
	}
	return s, nil
}

// Rename gives the seam a new id, keeping the old one under was. Its
// provider follows the new id: none when it names _.
func Rename(s *Seam, id string) {
	s.Was = append(s.Was, s.ID)
	s.ID = id
	parsed, err := ParseID(id)
	switch {
	case err != nil:
		// Left for validation to report.
	case parsed.Provider == NoProvider:
		s.Provider = nil
	case s.Provider == nil:
		s.Provider = &Party{Name: parsed.Provider}
	default:
		s.Provider.Name = parsed.Provider
	}
}

// Reject remembers a candidate that isn't a seam, by its id and its
// evidence's signal and file, and says whether it wasn't remembered already.
func (r *Registry) Reject(rj Rejection) (bool, error) {
	if _, err := ParseID(rj.ID); err != nil {
		return false, err
	}
	if r.Rejects(rj) {
		return false, nil
	}
	r.Rejected = append(r.Rejected, rj)
	return true, nil
}

// Rejects says whether the team rejected the candidate: the same id, from
// the same signal, in the same file.
func (r *Registry) Rejects(candidate Rejection) bool {
	for _, rj := range r.Rejected {
		if rj.ID == candidate.ID && rj.Signal == candidate.Signal && rj.File == candidate.File {
			return true
		}
	}
	return false
}

// problemsWith is an error listing the registry's problems about the seam, if
// it has any.
func (r *Registry) problemsWith(id string) error {
	var msgs []string
	for _, p := range r.Validate().Problems {
		if p.Seam == id {
			msgs = append(msgs, p.Message)
		}
	}
	if len(msgs) == 0 {
		return nil
	}
	return fmt.Errorf("%s %s", id, strings.Join(msgs, "; "))
}

func cloneSeam(s Seam) Seam {
	c := s
	c.Was = append([]string(nil), s.Was...)
	c.Consumers = append([]Party(nil), s.Consumers...)
	c.TestWorld = append([]ContractTest(nil), s.TestWorld...)
	if s.Provider != nil {
		p := *s.Provider
		c.Provider = &p
	}
	if s.Contract != nil {
		k := *s.Contract
		c.Contract = &k
	}
	return c
}
