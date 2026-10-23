// Package registry keeps a repo's seams: each seam's id, its provider and
// consumers, its audience, its contract and its test world, in a JSON file
// with a schema of its own.
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bitshepherds/sheepdog-seams/schemas"
)

// DefaultPath is where a repo keeps its registry, unless --registry says.
const DefaultPath = ".sheepdog/seams/registry.json"

// Schema names the registry file's schema.
const Schema = "registry"

// Registry is a repo's seams, and the candidates it rejected.
type Registry struct {
	Schema   string      `json:"schema"`
	Seams    []Seam      `json:"seams"`
	Rejected []Rejection `json:"rejected,omitempty"`
}

// Seam is one contract boundary.
type Seam struct {
	ID                   string         `json:"id"`
	Was                  []string       `json:"was,omitempty"`
	Provider             *Party         `json:"provider"`
	Consumers            []Party        `json:"consumers"`
	Audience             string         `json:"audience"`
	Contract             *Contract      `json:"contract,omitempty"`
	DeploysIndependently *bool          `json:"deploysIndependently,omitempty"`
	TestWorld            []ContractTest `json:"testWorld,omitempty"`
}

// Party is a seam's provider or one of its consumers.
type Party struct {
	Name     string `json:"name"`
	External bool   `json:"external,omitempty"`
	InRepo   bool   `json:"inRepo,omitempty"`
}

// Contract says where a seam's contract lives, and who owns it.
type Contract struct {
	Location string `json:"location"`
	Owner    string `json:"owner,omitempty"`
}

// ContractTest is a contract test on one side of a seam.
type ContractTest struct {
	Side string `json:"side"`
	Test string `json:"test"`
	Tier string `json:"tier"`
	Tool string `json:"tool,omitempty"`
}

// Rejection is a candidate the team decided isn't a seam, identified by its
// id and its evidence's signal kind and file.
type Rejection struct {
	ID     string `json:"id"`
	Signal string `json:"signal"`
	File   string `json:"file"`
	Reason string `json:"reason,omitempty"`
}

// Audiences a seam can have.
const (
	Internal = "internal"
	Public   = "public"
)

// New is an empty registry.
func New() *Registry { return &Registry{Schema: schemas.ID(Schema), Seams: []Seam{}} }

// Parse reads a registry, refusing one that doesn't match its schema.
func Parse(data []byte) (*Registry, error) {
	if err := schemas.Validate(Schema, data); err != nil {
		return nil, fmt.Errorf("the registry doesn't match its schema: %w", err)
	}
	var r Registry
	if err := json.Unmarshal(data, &r); err != nil { // coverage-ignore: the schema has already passed it
		return nil, err
	}
	return &r, nil
}

// Load reads the registry at path, or an empty one if there's no file yet.
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the registry's path is the user's to choose
	if errors.Is(err, fs.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return nil, err
	}
	r, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return r, nil
}

// Save writes the registry to path, making its directory if need be.
func (r *Registry) Save(path string) error {
	data, _ := json.MarshalIndent(r, "", "  ") // a Registry always marshals
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// Find is the seam with the id, or one it had before a rename.
func (r *Registry) Find(id string) (*Seam, bool) {
	for i := range r.Seams {
		s := &r.Seams[i]
		if s.ID == id {
			return s, true
		}
		for _, was := range s.Was {
			if was == id {
				return s, true
			}
		}
	}
	return nil, false
}

// Role is a party's part in one seam.
type Role struct {
	Seam string
	// Provides is true for the seam's provider, false for a consumer.
	Provides bool
}

// Roles lists the seams a party takes part in, and its part in each.
func (r *Registry) Roles(party string) []Role {
	var roles []Role
	for _, s := range r.Seams {
		if s.Provider != nil && s.Provider.Name == party {
			roles = append(roles, Role{Seam: s.ID, Provides: true})
		}
		for _, c := range s.Consumers {
			if c.Name == party {
				roles = append(roles, Role{Seam: s.ID})
			}
		}
	}
	return roles
}
