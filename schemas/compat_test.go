package schemas

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

const v1 = `{"$id": "https://example.com/schemas/thing/v1", "type": "object", "required": ["a"],
	"properties": {
		"a": {"type": "string"},
		"b": {"enum": ["x", "y"]},
		"c": {"const": 1},
		"list": {"type": "array", "items": {"type": "object", "properties": {"d": {"type": "string"}}}},
		"either": {"oneOf": [{"type": "string"}, {"type": "null"}]}
	},
	"$defs": {"e": {"type": "number"}}}`

func TestSchemaThatRemovesAFieldFailsUnlessItsMajorVersionRises(t *testing.T) {
	removed := strings.Replace(v1, `"a": {"type": "string"},`, ``, 1)
	breaks, err := Breaks([]byte(v1), []byte(removed))
	if err != nil || len(breaks) != 1 || !strings.Contains(breaks[0], "removes a") {
		t.Fatalf("Breaks = %q, %v", breaks, err)
	}
	major := strings.Replace(removed, "/thing/v1", "/thing/v2", 1)
	if breaks, err := Breaks([]byte(v1), []byte(major)); err != nil || len(breaks) != 0 {
		t.Fatalf("with a new major version, Breaks = %q, %v", breaks, err)
	}
}

func TestSchemaChangesThatBreakReadersAreFound(t *testing.T) {
	for change, want := range map[[2]string]string{
		{`"a": {"type": "string"}`, `"a": {"type": "number"}`}:                           "/properties/a changes type",
		{`"enum": ["x", "y"]`, `"enum": ["x"]`}:                                          "/properties/b drops the value y",
		{`"const": 1`, `"const": 2`}:                                                     "/properties/c changes its constant",
		{`"required": ["a"]`, `"required": ["a", "b"]`}:                                  "the document makes b required",
		{`"d": {"type": "string"}`, `"d": {"type": "boolean"}`}:                          "/properties/list/items/properties/d changes type",
		{`{"type": "string"}, {"type": "null"}`, `{"type": "number"}, {"type": "null"}`}: "/properties/either/oneOf/0 changes type",
		{`[{"type": "string"}, {"type": "null"}]`, `[{"type": "string"}]`}:               "drops oneOf option 1",
		{`"e": {"type": "number"}`, `"f": {"type": "number"}`}:                           "removes e",
	} {
		breaks, err := Breaks([]byte(v1), []byte(strings.Replace(v1, change[0], change[1], 1)))
		if err != nil || len(breaks) != 1 || !strings.Contains(breaks[0], want) {
			t.Errorf("%s → %s: Breaks = %q, %v; want %q", change[0], change[1], breaks, err, want)
		}
	}
}

func TestAddingAnOptionalFieldOrAnEnumValueBreaksNothing(t *testing.T) {
	added := strings.Replace(strings.Replace(v1, `"enum": ["x", "y"]`, `"enum": ["x", "y", "z"]`, 1),
		`"a": {"type": "string"},`, `"a": {"type": "string"}, "z": {"type": "string"},`, 1)
	if breaks, err := Breaks([]byte(v1), []byte(added)); err != nil || len(breaks) != 0 {
		t.Fatalf("Breaks = %q, %v", breaks, err)
	}
}

func TestBreaksRefusesASchemaThatIsNotJSON(t *testing.T) {
	if _, err := Breaks([]byte("{"), []byte(v1)); err == nil || !strings.Contains(err.Error(), "old") {
		t.Errorf("old: %v", err)
	}
	if _, err := Breaks([]byte(v1), []byte("{")); err == nil || !strings.Contains(err.Error(), "new") {
		t.Errorf("new: %v", err)
	}
}

// TestSchemasAreCompatibleWithTheLastRelease compares each schema with the
// one in the last release tag, from git. A schema the release didn't have
// is new, so it can't break anything.
func TestSchemasAreCompatibleWithTheLastRelease(t *testing.T) {
	tag, err := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "v*").Output()
	if err != nil {
		t.Skip("no release tag yet")
	}
	release := strings.TrimSpace(string(tag))
	for _, name := range Names() {
		old, err := exec.Command("git", "show", release+":schemas/"+name+".json").Output()
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			continue // new since the release
		}
		if err != nil {
			t.Fatal(err)
		}
		now, _ := Source(name)
		breaks, err := Breaks(old, now)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range breaks {
			t.Errorf("%s breaks %s's schema from %s: %s; raise its major version", name, name, release, b)
		}
	}
}

// TestBreaksAllowsOnlyAMoveOfAddress: the repo's rename moves every schema
// from bitshepherds/sheepdog-seams to bitshepherds/seams, still v1. That move
// alone breaks nothing, but it doesn't hide a real break made with it.
func TestBreaksAllowsOnlyAMoveOfAddress(t *testing.T) {
	const old = `{"$id": "https://github.com/bitshepherds/sheepdog-seams/schemas/thing/v1", "type": "object",
		"properties": {
			"schema": {"const": "https://github.com/bitshepherds/sheepdog-seams/schemas/thing/v1"},
			"a": {"type": "string"},
			"other": {"$ref": "https://github.com/bitshepherds/sheepdog-seams/schemas/registry/v1#/$defs/id"}
		}}`
	moved := strings.ReplaceAll(old, "https://github.com/bitshepherds/sheepdog-seams/schemas/", "https://github.com/bitshepherds/seams/schemas/")
	if breaks, err := Breaks([]byte(old), []byte(moved)); err != nil || len(breaks) != 0 {
		t.Fatalf("a move of address alone: Breaks = %q, %v", breaks, err)
	}
	dropped := strings.Replace(moved, `"a": {"type": "string"},`, ``, 1)
	breaks, err := Breaks([]byte(old), []byte(dropped))
	if err != nil || len(breaks) != 1 || !strings.Contains(breaks[0], "removes a") {
		t.Fatalf("a move that also drops a property: Breaks = %q, %v", breaks, err)
	}
	elsewhere := strings.ReplaceAll(old, "https://github.com/bitshepherds/sheepdog-seams/schemas/", "https://example.com/seams/schemas/")
	breaks, err = Breaks([]byte(old), []byte(elsewhere))
	if err != nil || len(breaks) != 1 || !strings.Contains(breaks[0], "/properties/schema changes its constant") {
		t.Fatalf("a move to any other address: Breaks = %q, %v", breaks, err)
	}
}
