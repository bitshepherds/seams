package detect

import (
	"regexp"
	"strings"
)

// Signals read from manifests.
const (
	sigHTTPServer     = "http-server"
	sigBroker         = "broker"
	sigSchemaRegistry = "schema-registry"
	sigPactLibrary    = "pact-library"
	sigVendor         = "vendor"
	sigOrgClient      = "org-client"
	sigWorkspace      = "workspace"
)

// clientPackage is a dependency named as a client or SDK for a service, such
// as @acme/pricing-client or github.com/acme/pricing-sdk.
var clientPackage = regexp.MustCompile(`^(.+?)[-_](?:client|sdk)$`)

func (s *scan) manifestSignals() {
	s.readDeps()
	modules := map[string]string{} // a component's full name → the component
	for _, c := range s.comps {
		if m := s.moduleName(c); m != "" {
			modules[m] = c.Name
		}
	}
	for _, c := range s.comps {
		s.serverFacts(c.Name)
		s.supportFacts(c.Name)
		for _, d := range s.deps[c.Name] {
			if s.vendorDep(c.Name, d) {
				continue
			}
			if lib, ok := modules[d.name]; ok && lib != c.Name {
				s.consume("lib", lib, generic, c.Name, high, s.evidence(sigWorkspace, consumes, c.Name, d.file, d.line))
				continue
			}
			s.orgClient(c, d)
		}
	}
}

// serverFacts: a server framework makes the component a provider.
func (s *scan) serverFacts(comp string) {
	if d, ok := s.depends(comp, httpServers...); ok {
		s.provide("http", comp, generic, medium, s.evidence(sigHTTPServer, provides, comp, d.file, d.line))
	}
	if d, ok := s.depends(comp, graphqlServers...); ok {
		s.provide("graphql", comp, generic, medium, s.evidence(sigHTTPServer, provides, comp, d.file, d.line))
	}
}

// supportFacts: a broker's, a schema registry's or Pact's library makes the
// component's seams of their kinds likelier.
func (s *scan) supportFacts(comp string) {
	for _, lib := range []struct {
		signal string
		names  []string
		kinds  []string
	}{
		{sigBroker, brokers, []string{"event", "queue"}},
		{sigSchemaRegistry, schemaRegistries, []string{"event"}},
		{sigPactLibrary, pactLibs, []string{"http", "event"}},
	} {
		if d, ok := s.depends(comp, lib.names...); ok {
			s.supports = append(s.supports, support{comp, lib.kinds, s.evidence(lib.signal, supports, comp, d.file, d.line)})
		}
	}
}

// vendorDep: a vendor's SDK makes the component its consumer.
func (s *scan) vendorDep(comp string, d dep) bool {
	for _, v := range vendors {
		for _, n := range v.deps {
			if d.name == n || strings.HasPrefix(d.name, n+"/") {
				s.consumeVendor(v.name, comp, medium, s.evidence(sigVendor, consumes, comp, d.file, d.line))
				return true
			}
		}
	}
	return false
}

// orgClient: a dependency on the org's own client or SDK for a service makes
// the component that service's consumer. The org is the npm scope or the Go
// module's owner the component shares; Python has none to share.
func (s *scan) orgClient(c Component, d dep) {
	m := clientPackage.FindStringSubmatch(lastElement(d.name))
	if m == nil || org(d.name) != org(s.moduleName(c)) || notServices[slug(m[1])] {
		return
	}
	s.consume("http", slug(m[1]), generic, c.Name, medium, s.evidence(sigOrgClient, consumes, c.Name, d.file, d.line))
}

// lastElement is a package's own name, without its scope or module path.
func lastElement(name string) string {
	name = majorVersion.ReplaceAllString(name, "")
	return name[strings.LastIndexByte(name, '/')+1:]
}

// org is the scope of an npm name, or the host and owner of a Go module path.
func org(name string) string {
	parts := strings.Split(name, "/")
	switch {
	case strings.HasPrefix(name, "@"):
		return parts[0]
	case len(parts) >= 3:
		return parts[0] + "/" + parts[1]
	}
	return ""
}

// provide records that a party provides a seam.
func (s *scan) provide(kind, party, name string, conf int, ev Evidence) {
	s.facts = append(s.facts, fact{kind: kind, name: name, provider: party, party: party, conf: conf, ev: ev})
}

// own records that a party provides a seam, unless other evidence names its
// provider: a topic's owner in IaC, say, when the code shows who publishes.
func (s *scan) own(kind, party, name string, conf int, ev Evidence) {
	s.facts = append(s.facts, fact{kind: kind, name: name, owner: party, party: party, conf: conf, ev: ev})
}

// consume records that a party consumes a seam, from a provider that's empty
// when the evidence doesn't say.
func (s *scan) consume(kind, provider, name, party string, conf int, ev Evidence) {
	s.facts = append(s.facts, fact{kind: kind, name: name, provider: provider, party: party, conf: conf, ev: ev})
}

// consumeVendor records that a party calls a vendor's API.
func (s *scan) consumeVendor(v, party string, conf int, ev Evidence) {
	s.facts = append(s.facts, fact{kind: "http", name: generic, provider: v, party: party, external: true, conf: conf, ev: ev})
}
