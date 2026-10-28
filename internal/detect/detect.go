// Package detect proposes a repo's seams from what its files say: its
// manifests, the patterns in its code, its contracts and its infrastructure
// as code. It reads text, with no parser for any language, so it is fast,
// and it proposes rather than decides: each candidate comes with its
// evidence, by file and line, and a confidence.
package detect

import (
	"io/fs"
	"slices"
	"strings"

	"github.com/bitshepherds/seams/internal/registry"
)

// Evidence is one thing in the repo that suggests a seam.
type Evidence struct {
	Signal string `json:"signal"`
	// Role is the party's part: provides, consumes, owns (provides, unless
	// the code says someone else does) or supports (makes a seam likelier).
	Role  string `json:"role"`
	Party string `json:"party"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	Text  string `json:"text"`
}

// Candidate is a seam the repo seems to have, for a person to confirm.
type Candidate struct {
	ID         string           `json:"id"`
	Kind       string           `json:"kind"`
	Provider   *registry.Party  `json:"provider"`
	Consumers  []registry.Party `json:"consumers"`
	Audience   string           `json:"audience"`
	Contract   string           `json:"contract,omitempty"`
	Confidence string           `json:"confidence"`
	Evidence   []Evidence       `json:"evidence"`
	Notes      []string         `json:"notes"`
}

// Component is a part of the repo that can take part in a seam: a directory
// with a manifest, named by it, or a top-level directory with none.
type Component struct {
	Name string `json:"name"`
	Dir  string `json:"dir"`
	// Manifest is the file it's named by, or empty when it's named by its
	// directory.
	Manifest string `json:"manifest,omitempty"`
}

// Result is what detection found.
type Result struct {
	Components []Component `json:"components"`
	Candidates []Candidate `json:"candidates"`
}

// Signals lists every kind of evidence, as the detect schema publishes them.
var Signals = []string{
	sigCatalog, sigPact, sigPactLibrary, sigContract, sigGenerator, sigCompatCheck,
	sigHTTPServer, sigHTTPRoute, sigHTTPClient, sigRouteCall, sigGRPC, sigOrgClient, sigWorkspace, sigVendor,
	sigBroker, sigSchemaRegistry, sigMessaging, sigEnv,
	sigIaCResource, sigIaCGateway, sigIaCSubscription, sigRemoteState, sigServerless,
	sigMigrations, sigDBClient, sigConfigSchema,
}

// Roles evidence can have.
const (
	provides = "provides"
	consumes = "consumes"
	owns     = "owns"
	supports = "supports"
)

// Confidences, lowest first.
var confidences = []string{"low", "medium", "high"}

const (
	low = iota
	medium
	high
)

// generic is the name of a component's one API of a kind, such as its HTTP
// API, when the evidence doesn't name it.
const generic = "api"

// fact is one piece of evidence, read as a part in one seam.
type fact struct {
	kind, name string
	// provider is who provides the seam, or empty when the evidence doesn't
	// say; owner provides it only if no evidence names a provider.
	provider, owner string
	// party is the component or vendor the evidence is about; a consumer when
	// it isn't the provider.
	party    string
	external bool // the provider is outside the system, such as a vendor
	public   bool
	contract string
	note     string
	conf     int
	ev       Evidence
}

// support is evidence that makes a component's seams of some kinds likelier,
// without naming one, such as a broker's client library.
type support struct {
	party string
	kinds []string
	ev    Evidence
}

// Detect proposes the seams of the repo in fsys, whose root directory is
// named root.
func Detect(fsys fs.FS, root string) (*Result, error) {
	s, err := newScan(fsys, root)
	if err != nil {
		return nil, err
	}
	s.manifestSignals()
	s.declaredSignals()
	s.iacSignals()
	s.codeSignals()
	s.dataSignals()
	s.resolve()
	return &Result{Components: s.comps, Candidates: s.candidates()}, nil
}

// Proposed is the candidates the registry hasn't rejected: a rejection
// names the candidate's id, and the signal and file of any of its evidence.
func Proposed(cands []Candidate, r *registry.Registry) []Candidate {
	out := []Candidate{}
	for _, c := range cands {
		if !slices.ContainsFunc(c.Evidence, func(e Evidence) bool {
			return r.Rejects(registry.Rejection{ID: c.ID, Signal: e.Signal, File: e.File})
		}) {
			out = append(out, c)
		}
	}
	return out
}

// slug makes s a name a seam's id can hold: lower case, with anything other
// than letters, digits, '.', '_' and '-' made '-'.
func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "._-")
}

// kebab makes a CamelCase name, such as a gRPC service's, a slug:
// PricingService is pricing-service.
func kebab(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' && s[i-1] >= 'a' && s[i-1] <= 'z' {
			b.WriteByte('-')
		}
		b.WriteRune(r)
	}
	return slug(b.String())
}
