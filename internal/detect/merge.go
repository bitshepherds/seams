package detect

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/bitshepherds/sheepdog-seams/internal/registry"
)

// key is a seam as the evidence names it so far.
type key struct{ kind, provider, name string }

// candidates merges the facts into seams, and reads each one's provider,
// consumers and confidence from its facts.
func (s *scan) candidates() []Candidate {
	groups := map[key][]fact{}
	for _, f := range s.facts {
		k := key{f.kind, f.provider, f.name}
		groups[k] = append(groups[k], f)
	}
	joinGeneric(groups)
	unowned := joinUnknown(groups)
	out := []Candidate{}
	for _, k := range sortedGroupKeys(groups) {
		if c, ok := s.candidate(k, groups[k], unowned[k]); ok {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sortedGroupKeys[V any](groups map[key]V) []key {
	keys := make([]key, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		return a.kind+":"+a.provider+"/"+a.name < b.kind+":"+b.provider+"/"+b.name
	})
	return keys
}

// joinGeneric merges a provider's unnamed API of a kind into its one named
// API of that kind, when it has exactly one.
func joinGeneric(groups map[key][]fact) {
	for _, k := range sortedGroupKeys(groups) {
		if k.name != generic || k.provider == "" {
			continue
		}
		var named []key
		for _, o := range sortedGroupKeys(groups) {
			if o.kind == k.kind && o.provider == k.provider && o.name != generic {
				named = append(named, o)
			}
		}
		if len(named) == 1 {
			groups[named[0]] = append(groups[named[0]], groups[k]...)
			delete(groups, k)
		}
	}
}

// Why a seam has no provider, though its evidence may name an owner.
const (
	sharedSeam  = "shared"  // several parties provide it, so none does alone
	unknownSeam = "unknown" // several might, and the evidence doesn't say which
)

// joinUnknown merges evidence that doesn't name a seam's provider into the
// seam of that kind and name whose provider is known: the one there is, or
// the one that is its owner. A topic or queue that several parties provide
// has no single owner, so their evidence is merged into one seam with none.
// It returns the seams left with no provider, and why.
func joinUnknown(groups map[key][]fact) map[key]string {
	byName := map[key][]key{} // a kind and name → the seams of it with a known provider
	for _, k := range sortedGroupKeys(groups) {
		if k.provider != "" && k.name != generic {
			n := key{k.kind, "", k.name}
			byName[n] = append(byName[n], k)
		}
	}
	unowned := map[key]string{}
	for _, n := range sortedGroupKeys(byName) {
		known := byName[n]
		facts, ok := groups[n]
		owner := key{n.kind, ownerOf(facts), n.name}
		switch {
		case n.kind == "event" || n.kind == "queue":
			if len(known) == 1 {
				break
			}
			for _, o := range known {
				groups[n] = append(groups[n], groups[o]...)
				delete(groups, o)
			}
			unowned[n] = sharedSeam
			continue
		case !ok:
			continue
		case slices.Contains(known, owner):
			known = []key{owner}
		case len(known) > 1:
			unowned[n] = unknownSeam
			continue
		}
		if ok {
			groups[known[0]] = append(groups[known[0]], facts...)
			delete(groups, n)
		}
	}
	return unowned
}

// ownerOf is the facts' one owner, or empty.
func ownerOf(facts []fact) string {
	owner := ""
	for _, f := range facts {
		if f.owner != "" && owner != "" && f.owner != owner {
			return ""
		}
		if f.owner != "" {
			owner = f.owner
		}
	}
	return owner
}

// candidate reads one seam from its facts, or says it isn't one: a party
// using its own API, or a table nobody else reads. An unowned seam has no
// provider, and says why.
func (s *scan) candidate(k key, facts []fact, unowned string) (Candidate, bool) {
	provider := s.providerOf(k, facts)
	if unowned != "" {
		provider = nil
	}
	c := Candidate{Kind: k.kind, Provider: provider, Consumers: []registry.Party{}, Audience: registry.Internal, Notes: []string{}}
	providerName := registry.NoProvider
	if provider != nil {
		providerName = provider.Name
	}
	c.ID = registry.ID{Kind: k.kind, Provider: providerName, Name: k.name}.String()
	providing, best := false, low
	signals := map[string]bool{}
	for _, f := range facts {
		best = max(best, f.conf)
		signals[f.ev.Signal] = true
		c.Evidence = append(c.Evidence, f.ev)
		if f.public {
			c.Audience = registry.Public
		}
		if f.contract != "" && (c.Contract == "" || f.contract < c.Contract) {
			c.Contract = f.contract
		}
		if f.note != "" && !slices.Contains(c.Notes, f.note) {
			c.Notes = append(c.Notes, f.note)
		}
		if f.ev.Role == provides || f.ev.Role == owns {
			providing = true
		} else if f.party != providerName && !slices.ContainsFunc(c.Consumers, func(p registry.Party) bool { return p.Name == f.party }) {
			c.Consumers = append(c.Consumers, registry.Party{Name: f.party, InRepo: s.inRepo(f.party)})
		}
	}
	if len(c.Consumers) == 0 && (!providing || k.kind == "table") {
		return c, false
	}
	sort.Slice(c.Consumers, func(i, j int) bool { return c.Consumers[i].Name < c.Consumers[j].Name })
	c.Evidence = append(c.Evidence, s.supporting(c)...)
	for _, e := range c.Evidence {
		signals[e.Signal] = true
	}
	c.Evidence = dedupe(c.Evidence)
	c.Confidence = confidences[confidence(best, len(signals), provider)]
	c.Notes = append(c.Notes, s.spread(c)...)
	if unowned == sharedSeam {
		c.Notes = append(c.Notes, "provided by more than one party, so it has no single owner: "+strings.Join(providedBy(facts), ", "))
	} else if provider == nil && len(c.Notes) == 0 {
		c.Notes = append(c.Notes, "the evidence doesn't say who provides it")
	}
	return c, true
}

// providedBy lists the parties the facts say provide the seam.
func providedBy(facts []fact) []string {
	var out []string
	for _, f := range facts {
		if f.provider != "" && f.ev.Role == provides && !slices.Contains(out, f.provider) {
			out = append(out, f.provider)
		}
	}
	sort.Strings(out)
	return out
}

// providerOf is the seam's provider: the one its evidence names, or failing
// that, its one owner, or nil.
func (s *scan) providerOf(k key, facts []fact) *registry.Party {
	name := k.provider
	external := false
	owners := map[string]bool{}
	for _, f := range facts {
		external = external || f.external
		if f.owner != "" {
			owners[f.owner] = true
		}
	}
	if name == "" && len(owners) == 1 {
		name = sortedKeys(owners)[0]
	}
	if name == "" {
		return nil
	}
	return &registry.Party{Name: name, External: external, InRepo: s.inRepo(name)}
}

// confidence is the strongest evidence's, one higher when two or more
// signals agree, and at most medium when nobody is known to provide it.
func confidence(best, signals int, provider *registry.Party) int {
	if signals > 1 {
		best = min(best+1, high)
	}
	if provider == nil {
		best = min(best, medium)
	}
	return best
}

// supporting is the evidence that makes the seam likelier without naming
// it, such as a party's broker library.
func (s *scan) supporting(c Candidate) []Evidence {
	parties := map[string]bool{}
	if c.Provider != nil {
		parties[c.Provider.Name] = true
	}
	for _, p := range c.Consumers {
		parties[p.Name] = true
	}
	var out []Evidence
	for _, sp := range s.supports {
		if parties[sp.party] && slices.Contains(sp.kinds, c.Kind) {
			out = append(out, sp.ev)
		}
	}
	return out
}

// dedupe sorts evidence by file and line, keeping one of each.
func dedupe(evs []Evidence) []Evidence {
	sort.SliceStable(evs, func(i, j int) bool {
		if evs[i].File != evs[j].File {
			return evs[i].File < evs[j].File
		}
		if evs[i].Line != evs[j].Line {
			return evs[i].Line < evs[j].Line
		}
		return evs[i].Signal < evs[j].Signal
	})
	return slices.CompactFunc(evs, func(a, b Evidence) bool {
		return a.File == b.File && a.Line == b.Line && a.Signal == b.Signal && a.Party == b.Party
	})
}

// spread notes a third party called from more than one module, each site
// named: with no SDK of the repo's own around it, each module depends on
// the vendor's API directly.
func (s *scan) spread(c Candidate) []string {
	if c.Provider == nil || !c.Provider.External {
		return nil
	}
	modules := map[string]bool{}
	var sites []string
	for _, e := range c.Evidence {
		if e.Signal == sigVendor && sourceExts[path.Ext(e.File)] {
			modules[path.Dir(e.File)] = true
			sites = append(sites, fmt.Sprintf("%s:%d", e.File, e.Line))
		}
	}
	if len(modules) < 2 {
		return nil
	}
	return []string{fmt.Sprintf("%s is called from more than one module, with no SDK of the repo's own around it: %s",
		c.Provider.Name, strings.Join(sites, ", "))}
}
