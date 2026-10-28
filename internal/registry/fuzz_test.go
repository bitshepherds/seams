package registry

import "testing"

// FuzzParseID: an id either parses and prints back as it was read, or is
// refused; nothing panics.
func FuzzParseID(f *testing.F) {
	for _, s := range []string{
		"event:backend/quote-accepted", "table:_/quotes", "http:stripe/payments",
		"lib:acme-client/orders", "quote-accepted", "event:/x", "rpc:a/b", ":/", "",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		id, err := ParseID(s)
		if err == nil && id.String() != s {
			t.Fatalf("ParseID(%q).String() = %q", s, id.String())
		}
	})
}

// FuzzLoadRegistry: any input is either refused or gives a registry that
// validates without panicking.
func FuzzLoadRegistry(f *testing.F) {
	for _, s := range []string{
		`{"schema": "https://github.com/bitshepherds/seams/schemas/registry/v1", "seams": []}`,
		`{"schema": "https://github.com/bitshepherds/seams/schemas/registry/v1", "seams": [` +
			`{"id": "table:_/quotes", "provider": null, "consumers": [{"name": "a"}], "audience": "internal"}]}`,
		`{"seams": [{"id": "x"}]}`, `[]`, `{`, ``,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if r, err := Parse(data); err == nil {
			r.Validate()
		}
	})
}
