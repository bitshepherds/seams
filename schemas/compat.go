package schemas

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"
)

// Breaks lists how a new version of a schema breaks documents or readers
// that the old one served: a removed property, a changed type or constant,
// a newly required property, a dropped enum value. A break is allowed only
// when the schema's major version rises, so Breaks returns none then.
func Breaks(old, new []byte) ([]string, error) {
	var o, n map[string]any
	if err := json.Unmarshal(old, &o); err != nil {
		return nil, fmt.Errorf("the old schema: %w", err)
	}
	if err := json.Unmarshal(new, &n); err != nil {
		return nil, fmt.Errorf("the new schema: %w", err)
	}
	oldID, _ := o["$id"].(string)
	newID, _ := n["$id"].(string)
	if Major(oldID) != Major(newID) {
		return nil, nil
	}
	var breaks []string
	compare("", o, n, &breaks)
	return breaks, nil
}

// compare walks two versions of one subschema, noting each break.
func compare(at string, o, n map[string]any, breaks *[]string) {
	note := func(format string, a ...any) { *breaks = append(*breaks, fmt.Sprintf(format, a...)) }
	if t, ok := o["type"]; ok && !reflect.DeepEqual(t, n["type"]) {
		note("%s changes type from %v to %v", where(at), t, n["type"])
	}
	if c, ok := o["const"]; ok && !reflect.DeepEqual(c, n["const"]) {
		note("%s changes its constant from %v to %v", where(at), c, n["const"])
	}
	if e, ok := o["enum"].([]any); ok {
		ne, _ := n["enum"].([]any)
		for _, v := range e {
			if !slices.ContainsFunc(ne, func(w any) bool { return reflect.DeepEqual(v, w) }) {
				note("%s drops the value %v", where(at), v)
			}
		}
	}
	oldRequired := stringList(o["required"])
	for _, r := range stringList(n["required"]) {
		if !slices.Contains(oldRequired, r) {
			note("%s makes %s required", where(at), r)
		}
	}
	for _, key := range []string{"properties", "$defs"} {
		op, _ := o[key].(map[string]any)
		np, _ := n[key].(map[string]any)
		for _, name := range sortedKeys(op) {
			next, ok := np[name].(map[string]any)
			if !ok {
				note("%s removes %s", where(at), name)
				continue
			}
			sub, _ := op[name].(map[string]any)
			compare(at+"/"+key+"/"+name, sub, next, breaks)
		}
	}
	if oi, ok := o["items"].(map[string]any); ok {
		ni, _ := n["items"].(map[string]any)
		compare(at+"/items", oi, ni, breaks)
	}
	for _, key := range []string{"oneOf", "anyOf"} {
		oa, _ := o[key].([]any)
		na, _ := n[key].([]any)
		for i := range oa {
			if i >= len(na) {
				note("%s drops %s option %d", where(at), key, i)
				continue
			}
			os, _ := oa[i].(map[string]any)
			ns, _ := na[i].(map[string]any)
			compare(fmt.Sprintf("%s/%s/%d", at, key, i), os, ns, breaks)
		}
	}
}

func where(at string) string {
	if at == "" {
		return "the document"
	}
	return at
}

func stringList(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, s := range list {
		if s, ok := s.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
