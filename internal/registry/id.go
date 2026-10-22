package registry

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Kinds are the kinds of seam an id can name.
var Kinds = []string{"http", "grpc", "graphql", "event", "queue", "file", "config", "table", "lib", "cli"}

// NoProvider stands for the provider in the id of a seam with no single owner.
const NoProvider = "_"

// IDForm is how an id is written, for messages.
const IDForm = "kind:provider/name"

var (
	nameForm = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]*[a-z0-9])?$`)
	idForm   = regexp.MustCompile(`^([a-z]+):([^/:]+)/([^/:]+)$`)
)

// ID is a seam's id, <kind>:<provider>/<name>. Consumers aren't part of it,
// so two repos that register the same seam give it the same id.
type ID struct {
	Kind, Provider, Name string
}

func (id ID) String() string { return id.Kind + ":" + id.Provider + "/" + id.Name }

// ParseID reads an id, refusing one that isn't kind:provider/name, with a
// known kind and lower-case names.
func ParseID(s string) (ID, error) {
	m := idForm.FindStringSubmatch(s)
	if m == nil {
		return ID{}, fmt.Errorf("%q isn't an id of the form %s", s, IDForm)
	}
	id := ID{Kind: m[1], Provider: m[2], Name: m[3]}
	switch {
	case !slices.Contains(Kinds, id.Kind):
		return ID{}, fmt.Errorf("%q has kind %q, which isn't one of %s", s, id.Kind, strings.Join(Kinds, ", "))
	case id.Provider != NoProvider && !ValidName(id.Provider):
		return ID{}, fmt.Errorf("%q has provider %q, which isn't lower-case letters, digits, '.', '_' or '-'", s, id.Provider)
	case !ValidName(id.Name):
		return ID{}, fmt.Errorf("%q has name %q, which isn't lower-case letters, digits, '.', '_' or '-'", s, id.Name)
	}
	return id, nil
}

// ValidName says whether s can name a party, or a seam within its provider.
func ValidName(s string) bool { return nameForm.MatchString(s) }
