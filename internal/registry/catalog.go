package registry

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Change is one thing an import did, or chose not to do, to one seam.
type Change struct {
	Seam    string `json:"seam"`
	Action  string `json:"action"` // added, filled, disagrees or skipped
	Message string `json:"message"`
}

// entity is the part of a Backstage catalog entity an import reads.
type entity struct {
	Kind     string
	Metadata struct{ Name string }
	Spec     struct {
		Type         string
		ProvidesApis []string `yaml:"providesApis"`
		ConsumesApis []string `yaml:"consumesApis"`
	}
}

// api is what the catalog says about one API.
type api struct {
	kind                 string
	providers, consumers []string
}

// apiKinds maps a Backstage API's type to a seam's kind.
var apiKinds = map[string]string{"openapi": "http", "asyncapi": "event", "grpc": "grpc", "graphql": "graphql"}

// ImportCatalog fills in seams' providers and consumers from a Backstage
// catalog-info.yaml: its components' providesApis and consumesApis. A seam is
// matched to an API by name. Where the two disagree, the registry wins, and
// the change says so.
func (r *Registry) ImportCatalog(data []byte) ([]Change, error) {
	apis, err := readCatalog(data)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(apis))
	for name := range apis {
		names = append(names, name)
	}
	sort.Strings(names)
	var changes []Change
	for _, name := range names {
		changes = append(changes, r.importAPI(name, apis[name])...)
	}
	return changes, nil
}

func readCatalog(data []byte) (map[string]*api, error) {
	apis := map[string]*api{}
	of := func(ref string) *api {
		name := entityName(ref)
		if apis[name] == nil {
			apis[name] = &api{kind: "http"}
		}
		return apis[name]
	}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	for {
		var e entity
		err := dec.Decode(&e)
		if errors.Is(err, io.EOF) {
			return apis, nil
		}
		if err != nil {
			return nil, fmt.Errorf("catalog-info: %w", err)
		}
		switch e.Kind {
		case "API":
			if kind, ok := apiKinds[e.Spec.Type]; ok {
				of(e.Metadata.Name).kind = kind
			}
		case "Component":
			component := entityName(e.Metadata.Name)
			for _, ref := range e.Spec.ProvidesApis {
				a := of(ref)
				a.providers = append(a.providers, component)
			}
			for _, ref := range e.Spec.ConsumesApis {
				a := of(ref)
				a.consumers = append(a.consumers, component)
			}
		}
	}
}

// entityName is a Backstage entity reference's name, such as quotes from
// api:default/quotes.
func entityName(ref string) string {
	if i := strings.LastIndexAny(ref, ":/"); i >= 0 {
		ref = ref[i+1:]
	}
	return strings.ToLower(ref)
}

func (r *Registry) importAPI(name string, a *api) []Change {
	for i := range r.Seams {
		if id, err := ParseID(r.Seams[i].ID); err == nil && id.Name == name {
			return r.fillIn(&r.Seams[i], a)
		}
	}
	if len(a.providers) != 1 || len(a.consumers) == 0 {
		return []Change{{name, "skipped", fmt.Sprintf(
			"catalog-info names %d provider(s) and %d consumer(s) of the API %s; a seam has one provider and a consumer", len(a.providers), len(a.consumers), name)}}
	}
	s := Seam{ID: ID{Kind: a.kind, Provider: a.providers[0], Name: name}.String(), Provider: &Party{Name: a.providers[0]}, Audience: Internal}
	for _, c := range a.consumers {
		s.Consumers = append(s.Consumers, Party{Name: c})
	}
	if err := r.Add(s); err != nil {
		return []Change{{s.ID, "skipped", err.Error()}}
	}
	return []Change{{s.ID, "added", "from catalog-info's API " + name}}
}

// fillIn adds the consumers catalog-info names that the seam lacks, and
// notes where catalog-info's provider differs from the registry's.
func (r *Registry) fillIn(s *Seam, a *api) []Change {
	var changes []Change
	for _, p := range a.providers {
		if s.Provider == nil || s.Provider.Name != p {
			changes = append(changes, Change{s.ID, "disagrees", fmt.Sprintf(
				"catalog-info says %s provides it, but the registry says %s; the registry's kept", p, providerOf(s))})
		}
	}
	var adding []string
	for _, c := range a.consumers {
		if !slices.ContainsFunc(s.Consumers, func(p Party) bool { return p.Name == c }) {
			adding = append(adding, c)
		}
	}
	if len(adding) == 0 {
		return changes
	}
	id := s.ID
	_, err := r.Update(id, func(s *Seam) {
		for _, c := range adding {
			s.Consumers = append(s.Consumers, Party{Name: c})
		}
	})
	if err != nil {
		return append(changes, Change{id, "disagrees", err.Error() + "; the registry's kept"})
	}
	return append(changes, Change{id, "filled", "consumers " + strings.Join(adding, ", ") + ", from catalog-info"})
}

func providerOf(s *Seam) string {
	if s.Provider == nil {
		return "it has no single owner"
	}
	return s.Provider.Name
}
