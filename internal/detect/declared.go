package detect

import (
	"encoding/json"
	"errors"
	"io"
	"path"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Signals that read what the repo declares.
const (
	sigCatalog     = "catalog"
	sigPact        = "pact"
	sigContract    = "contract"
	sigGenerator   = "generator"
	sigCompatCheck = "compat-check"
)

var (
	openapiKey   = regexp.MustCompile(`(?m)^(?:openapi|swagger):|^\s*"(?:openapi|swagger)"\s*:`)
	asyncapiKey  = regexp.MustCompile(`(?m)^asyncapi:|^\s*"asyncapi"\s*:`)
	protoService = regexp.MustCompile(`^\s*service\s+(\w+)\s*\{`)
	graphqlRoot  = regexp.MustCompile(`^\s*(?:extend\s+)?(?:schema\s*\{|type\s+(?:Query|Mutation|Subscription)\b)`)

	generatorRef = regexp.MustCompile("(?:\\binput|\\btarget|\"inputSpec\"|\\binputSpec|\"descriptionLocation\")\\s*[:=]\\s*[\"'`]([^\"'`]+\\.(?:ya?ml|json)|https?://[^\"'`]+)[\"'`]")
	compatTool   = regexp.MustCompile(`\b(?:oasdiff|openapi-diff|buf\s+breaking|graphql-inspector|optic\s+diff)\b`)
	specPath     = regexp.MustCompile(`(?:[\w./-]+:)?([\w./-]+\.(?:ya?ml|json|proto|graphql))`)
	canIDeploy   = regexp.MustCompile(`\bcan-i-deploy\b.*--pacticipant[= ]["']?([\w.-]+)`)
)

// generatorConfigs are client generators' configs: Orval's,
// openapi-generator's and Kiota's.
var generatorConfigs = []string{"orval.config.*", "openapitools.json", "*openapi-generator*.json", "*openapi-generator*.y*ml", "kiota-lock.json"}

func (s *scan) declaredSignals() {
	for _, f := range s.filesNamed("catalog-info.yaml", "catalog-info.yml") {
		s.catalog(f)
	}
	for _, f := range s.filesNamed("*.json") {
		if t := s.read(f); strings.Contains(t, `"consumer"`) && strings.Contains(t, `"provider"`) {
			s.pact(f)
		}
	}
	for _, f := range s.files {
		if !s.ci(f) {
			s.contractFile(f, low, nil)
		}
	}
	for _, f := range s.filesNamed(generatorConfigs...) {
		s.generator(f)
	}
	for _, f := range s.filesNamed("buf.gen.yaml") {
		comp := s.component(f)
		s.supports = append(s.supports, support{comp, []string{"grpc"}, s.evidence(sigGenerator, supports, comp, f, 1)})
	}
	for _, f := range s.files {
		if s.ci(f) {
			s.compatChecks(f)
		}
	}
}

// ci says whether a file is a CI pipeline's.
func (s *scan) ci(f string) bool {
	return strings.HasPrefix(f, ".github/workflows/") || path.Base(f) == ".gitlab-ci.yml"
}

// entity is the part of a Backstage catalog entity detection reads.
type entity struct {
	Kind     string
	Metadata struct{ Name string }
	Spec     struct {
		Type         string
		ProvidesApis []string `yaml:"providesApis"`
		ConsumesApis []string `yaml:"consumesApis"`
	}
}

// apiKinds maps a Backstage API's type to a seam's kind.
var apiKinds = map[string]string{"openapi": "http", "asyncapi": "event", "grpc": "grpc", "graphql": "graphql"}

// catalog reads Backstage's providesApis and consumesApis.
func (s *scan) catalog(f string) {
	var entities []entity
	dec := yaml.NewDecoder(strings.NewReader(s.read(f)))
	for {
		var e entity
		if err := dec.Decode(&e); err != nil {
			break // the end, or YAML the catalog can't use
		}
		entities = append(entities, e)
	}
	kinds := map[string]string{}
	for _, e := range entities {
		if k, ok := apiKinds[e.Spec.Type]; ok && e.Kind == "API" {
			kinds[entityName(e.Metadata.Name)] = k
		}
	}
	kindOf := func(api string) string {
		if k, ok := kinds[api]; ok {
			return k
		}
		return "http"
	}
	for _, e := range entities {
		comp := entityName(e.Metadata.Name)
		for _, ref := range e.Spec.ProvidesApis {
			api := entityName(ref)
			s.provide(kindOf(api), comp, api, high, s.evidence(sigCatalog, provides, comp, f, s.lineOf(f, ref)))
		}
		for _, ref := range e.Spec.ConsumesApis {
			api := entityName(ref)
			s.consume(kindOf(api), "", api, comp, high, s.evidence(sigCatalog, consumes, comp, f, s.lineOf(f, ref)))
		}
	}
}

// entityName is a Backstage entity reference's name, such as quotes from
// api:default/quotes.
func entityName(ref string) string {
	if i := strings.LastIndexAny(ref, ":/"); i >= 0 {
		ref = ref[i+1:]
	}
	return slug(ref)
}

// pact reads a Pact file: its consumer consumes its provider's API, or its
// events, for a message pact.
func (s *scan) pact(f string) {
	var p struct {
		Consumer, Provider struct{ Name string }
		Interactions       []json.RawMessage
		Messages           []json.RawMessage
	}
	if json.Unmarshal([]byte(s.read(f)), &p) != nil || p.Consumer.Name == "" || p.Provider.Name == "" {
		return
	}
	kind := "http"
	if len(p.Messages) > 0 {
		kind = "event"
	}
	provider, consumer := slug(p.Provider.Name), slug(p.Consumer.Name)
	s.provide(kind, provider, generic, high, s.evidence(sigPact, provides, provider, f, s.lineOf(f, `"provider"`)))
	s.consume(kind, provider, generic, consumer, high, s.evidence(sigPact, consumes, consumer, f, s.lineOf(f, `"consumer"`)))
	s.facts[len(s.facts)-2].contract = f
}

// spec is one API a contract file describes.
type spec struct {
	kind, name string
	line       int
}

// specs lists the APIs a contract file describes: a .proto's services, a
// GraphQL schema, or an OpenAPI or AsyncAPI document.
func (s *scan) specs(f string) []spec {
	switch path.Ext(f) {
	case ".proto":
		var out []spec
		for i, l := range s.lines(f) {
			if m := protoService.FindStringSubmatch(l); m != nil {
				out = append(out, spec{"grpc", kebab(m[1]), i + 1})
			}
		}
		return out
	case ".graphql", ".gql":
		for i, l := range s.lines(f) {
			if graphqlRoot.MatchString(l) {
				return []spec{{"graphql", generic, i + 1}}
			}
		}
	case ".json", ".yaml", ".yml":
		text := s.read(f)
		if loc := openapiKey.FindStringIndex(text); loc != nil {
			return []spec{{"http", generic, strings.Count(text[:loc[0]], "\n") + 1}}
		}
		if asyncapiKey.MatchString(text) {
			return s.channels(f)
		}
	}
	return nil
}

// channels are an AsyncAPI document's channels, each an event.
func (s *scan) channels(f string) []spec {
	var out []spec
	if ch := mapGet(s.yamlRoot(f), "channels"); ch != nil {
		for i := 0; i+1 < len(ch.Content); i += 2 {
			if name := slug(ch.Content[i].Value); name != "" {
				out = append(out, spec{"event", name, ch.Content[i].Line})
			}
		}
	}
	return out
}

// contractFile records the APIs a contract file describes, on the file's
// own evidence or, when check is given, a CI step that checks it. Its
// holder provides an HTTP or GraphQL API; a service or channel may be held
// in a shared place, so its holder only owns it.
func (s *scan) contractFile(f string, conf int, check *Evidence) {
	holder := s.component(f)
	for _, sp := range s.specs(f) {
		role := owns
		if sp.name == generic {
			role = provides
		}
		ev := s.evidence(sigContract, role, holder, f, sp.line)
		if check != nil {
			ev = *check
			ev.Role, ev.Party = role, holder
		}
		if role == provides {
			s.provide(sp.kind, holder, sp.name, conf, ev)
		} else {
			s.own(sp.kind, holder, sp.name, conf, ev)
		}
		s.facts[len(s.facts)-1].contract = f
	}
}

// generator reads a client generator's config: the component it's in
// consumes the API of the spec it generates from.
func (s *scan) generator(f string) {
	comp := s.component(f)
	for i, l := range s.lines(f) {
		m := generatorRef.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		ev := s.evidence(sigGenerator, consumes, comp, f, i+1)
		if host := serviceURL.FindStringSubmatch(`"` + m[1] + `"`); host != nil {
			s.consume("http", slug(host[1]), generic, comp, medium, ev)
		} else if spec, ok := s.resolvePath(f, m[1]); ok {
			s.consume("http", s.component(spec), generic, comp, high, ev)
		}
	}
}

// compatChecks reads a CI pipeline for API compatibility checks, each of
// which says the API it checks is a seam.
func (s *scan) compatChecks(f string) {
	for i, l := range s.lines(f) {
		if m := canIDeploy.FindStringSubmatch(l); m != nil {
			party := slug(m[1])
			s.supports = append(s.supports, support{party, []string{"http", "event"}, s.evidence(sigCompatCheck, supports, party, f, i+1)})
		}
		if !compatTool.MatchString(l) {
			continue
		}
		for _, m := range specPath.FindAllStringSubmatch(l, -1) {
			if p, ok := s.resolvePath("", m[1]); ok {
				ev := s.evidence(sigCompatCheck, provides, "", f, i+1)
				s.contractFile(p, medium, &ev)
			}
		}
	}
}

// yamlRoot is a YAML or JSON file's top-level mapping, or nil.
func (s *scan) yamlRoot(f string) *yaml.Node {
	var doc yaml.Node
	err := yaml.NewDecoder(strings.NewReader(s.read(f))).Decode(&doc)
	if err != nil && !errors.Is(err, io.EOF) || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	return doc.Content[0]
}

// mapGet is a mapping's value for a key, or nil.
func mapGet(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
