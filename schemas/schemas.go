// Package schemas holds the JSON Schemas of seams' public contract: the
// registry file, and each command's --json output. They're published with
// each release, and each one's major version is the last element of its $id.
package schemas

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed *.json
var files embed.FS

// Names lists every schema, by its file name without .json.
func Names() []string {
	entries, _ := fs.ReadDir(files, ".")
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), ".json"))
	}
	return names
}

// Source is a schema's JSON.
func Source(name string) ([]byte, error) { return files.ReadFile(name + ".json") }

// ID is the $id of the named schema, which documents written to it carry in
// their "schema" field.
func ID(name string) string { return compiled(name).id }

type entry struct {
	id     string
	schema *jsonschema.Schema
}

var cache sync.Map // name → entry

func compiled(name string) entry {
	if e, ok := cache.Load(name); ok {
		return e.(entry)
	}
	e, err := compile(name)
	if err != nil {
		// The schemas are embedded and tested, so this is a bug in the build.
		panic(fmt.Sprintf("schema %s: %v", name, err))
	}
	cache.Store(name, e)
	return e
}

func compile(name string) (entry, error) {
	src, err := Source(name)
	if err != nil {
		return entry{}, err
	}
	// Past here, nothing fails for a schema TestEverySchemaCompilesAndNamesItsMajorVersion passes.
	var head struct {
		ID string `json:"$id"`
	}
	if err := json.Unmarshal(src, &head); err != nil { // coverage-ignore: tested schemas are JSON
		return entry{}, err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(src))
	if err != nil { // coverage-ignore: tested schemas are JSON
		return entry{}, err
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(head.ID, doc); err != nil { // coverage-ignore: tested schemas have an $id
		return entry{}, err
	}
	// The other schemas too, which this one may refer to.
	for _, other := range Names() {
		if other != name {
			if err := addResource(c, other); err != nil { // coverage-ignore: tested schemas load
				return entry{}, err
			}
		}
	}
	sch, err := c.Compile(head.ID)
	if err != nil { // coverage-ignore: tested schemas compile
		return entry{}, err
	}
	return entry{id: head.ID, schema: sch}, nil
}

func addResource(c *jsonschema.Compiler, name string) error {
	src, _ := Source(name) // a name from Names
	var head struct {
		ID string `json:"$id"`
	}
	_ = json.Unmarshal(src, &head)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(src))
	if err != nil { // coverage-ignore: tested schemas are JSON
		return err
	}
	return c.AddResource(head.ID, doc)
}

// Validate checks a JSON document against the named schema.
func Validate(name string, doc []byte) error {
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	if err != nil {
		return fmt.Errorf("not JSON: %w", err)
	}
	return compiled(name).schema.Validate(v)
}

// Major is the major version in a schema $id, its last path element, such as
// "v1".
func Major(id string) string { return path.Base(id) }
