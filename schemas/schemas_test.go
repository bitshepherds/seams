package schemas

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestEverySchemaCompilesAndNamesItsMajorVersion(t *testing.T) {
	names := Names()
	if len(names) == 0 {
		t.Fatal("no schemas")
	}
	for _, name := range names {
		id := ID(name)
		if !strings.Contains(id, "/schemas/"+name+"/") || !strings.HasPrefix(Major(id), "v") {
			t.Errorf("%s has $id %q, which doesn't end /schemas/%s/v<major>", name, id, name)
		}
		if ID(name) != id {
			t.Errorf("%s: a second compile gave another $id", name)
		}
	}
}

func TestValidateRefusesWhatIsNotJSONOrDoesNotMatch(t *testing.T) {
	if err := Validate("registry", []byte("{")); err == nil || !strings.Contains(err.Error(), "not JSON") {
		t.Errorf("Validate({) = %v", err)
	}
	if err := Validate("registry", []byte(`{"seams": []}`)); err == nil {
		t.Error("a registry with no schema field passed")
	}
}

func TestAnUnknownSchemaIsABug(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("compiling an unknown schema didn't panic")
		}
	}()
	ID("nothing")
}

// TestSchemaIDsNameTheSeamsRepo checks every schema lives at the renamed
// repo's address, still at v1, and nothing in one names the old repo or folder.
func TestSchemaIDsNameTheSeamsRepo(t *testing.T) {
	for _, name := range Names() {
		src, _ := Source(name)
		var doc map[string]any
		if err := json.Unmarshal(src, &doc); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ids := []string{doc["$id"].(string)}
		props, _ := doc["properties"].(map[string]any)
		schema, _ := props["schema"].(map[string]any)
		if c, ok := schema["const"].(string); ok {
			ids = append(ids, c)
		} else {
			t.Errorf("%s has no schema const", name)
		}
		ids = append(ids, refs(doc)...)
		for _, id := range ids {
			address, _, _ := strings.Cut(id, "#")
			if !strings.HasPrefix(address, "https://github.com/bitshepherds/seams/schemas/") || !strings.HasSuffix(address, "/v1") {
				t.Errorf("%s names %q, not https://github.com/bitshepherds/seams/schemas/<name>/v1", name, id)
			}
		}
		for _, old := range []string{"sheepdog-seams", ".sheepdog/seams"} {
			if bytes.Contains(src, []byte(old)) {
				t.Errorf("%s mentions %s", name, old)
			}
		}
	}
}

// refs gathers every $ref to another schema, at any depth; a ref within
// the same schema starts "#" and names no address.
func refs(v any) []string {
	var out []string
	switch v := v.(type) {
	case map[string]any:
		for k, sub := range v {
			if s, ok := sub.(string); ok && k == "$ref" && !strings.HasPrefix(s, "#") {
				out = append(out, s)
			}
			out = append(out, refs(sub)...)
		}
	case []any:
		for _, sub := range v {
			out = append(out, refs(sub)...)
		}
	}
	return out
}
