package schemas

import (
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
