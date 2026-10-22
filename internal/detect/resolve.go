package detect

import (
	"strings"
)

// envSuffixes classify an env name by its ending, longest first: what kind
// of seam it names, and the stem that names the seam.
var envSuffixes = []struct{ suffix, kind string }{
	{"_QUEUE_URL", "queue"}, {"_QUEUE_NAME", "queue"}, {"_QUEUE_ARN", "queue"}, {"_QUEUE", "queue"},
	{"_TOPIC_ARN", "event"}, {"_TOPIC_NAME", "event"}, {"_TOPIC", "event"},
	{"_EVENT_BUS_NAME", "event"}, {"_EVENT_BUS", "event"}, {"_STREAM_NAME", "event"},
	{"_GRAPHQL_URL", "graphql"},
	{"_SERVICE_URL", "http"}, {"_API_URL", "http"}, {"_BASE_URL", "http"}, {"_URL", "http"},
}

// envPrefixes are frameworks' prefixes for env names a browser bundle may
// read, which name no service.
var envPrefixes = []string{"NEXT_PUBLIC_", "NUXT_PUBLIC_", "EXPO_PUBLIC_", "REACT_APP_", "VITE_", "PUBLIC_"}

// envKind is the kind of seam an env name names, and its stem, such as
// http and pricing for PRICING_URL; the kind is empty for none.
func envKind(name string) (kind, stem string) {
	for _, prefix := range envPrefixes {
		name = strings.TrimPrefix(name, prefix)
	}
	for _, e := range envSuffixes {
		if strings.HasSuffix(name, e.suffix) {
			return e.kind, slug(strings.ReplaceAll(strings.TrimSuffix(name, e.suffix), "_", "-"))
		}
	}
	return "", ""
}

// genericPaths are served by nearly every service, so a call to one names
// no seam.
var genericPaths = map[string]bool{
	"": true, "health": true, "healthz": true, "metrics": true, "ready": true, "readyz": true,
	"live": true, "livez": true, "status": true, "ping": true, "version": true,
}

// resolve matches what one part of the repo uses to what another declares:
// env names to IaC, calls to routes, table names to migrations, and imports
// to exports.
func (s *scan) resolve() {
	for _, u := range s.envs {
		s.resolveEnv(u)
	}
	for _, c := range s.calls {
		s.resolveCall(c)
	}
	for _, r := range s.tableRefs {
		if def, ok := s.tables[r.table]; ok && def.owner != r.comp {
			s.consume("table", def.owner, r.table, r.comp, medium, r.ev)
		}
	}
	for _, im := range s.imports {
		f := im.f
		if c, ok := s.exports[im.export]; ok {
			f.provider, f.conf = c, medium
		} else {
			f.note = "imported from a stack the repo doesn't export it from, perhaps another repo's"
		}
		s.facts = append(s.facts, f)
	}
}

// resolveEnv reads an env name through what IaC sets it to, or failing that,
// through its name.
func (s *scan) resolveEnv(u envUse) {
	evs := []Evidence{u.ev}
	t, ok := s.envTarget(u.comp, u.name)
	if ok {
		evs = append(evs, t.ev)
	}
	kind, stem := envKind(u.name)
	for _, ev := range evs {
		ev.Party = u.comp
		switch {
		case t.res != nil:
			s.messageFact(t.res.kind, t.res.name, u.direction, u.comp, high, withRole(ev))
		case t.comp != "":
			s.consume("http", t.comp, generic, u.comp, high, role(ev, consumes))
		case t.url != "":
			s.consumeURL(t.url, u.comp, role(ev, consumes))
		case kind == "event" || kind == "queue":
			s.messageFact(kind, stem, u.direction, u.comp, medium, withRole(ev))
		case kind != "" && !notServices[stem]:
			s.consumeNamed(kind, stem, u.comp, role(ev, consumes))
		}
	}
}

// envTarget is what IaC sets a component's env name to: in the component's
// own environment, or in the one environment that sets the name.
func (s *scan) envTarget(comp, name string) (envTarget, bool) {
	if t, ok := s.envMap[comp][name]; ok {
		return t, true
	}
	var found []envTarget
	for _, c := range sortedKeys(s.envMap) {
		if t, ok := s.envMap[c][name]; ok {
			found = append(found, t)
		}
	}
	if len(found) == 1 {
		return found[0], true
	}
	return envTarget{}, false
}

func role(ev Evidence, r string) Evidence {
	ev.Role = r
	return ev
}

func withRole(ev Evidence) func(string) Evidence {
	return func(r string) Evidence { return role(ev, r) }
}

// consumeNamed records a party calling the service a name names: a vendor,
// a component, or one outside the repo.
func (s *scan) consumeNamed(kind, name, party string, ev Evidence) {
	for _, v := range vendors {
		if v.name == name {
			s.consumeVendor(v.name, party, medium, ev)
			return
		}
	}
	provider, ok := s.named(name)
	conf := medium
	if !ok {
		provider, conf = name, low
	}
	s.consume(kind, provider, generic, party, conf, ev)
}

// consumeURL records a party calling a URL: a vendor's, or a service's by
// its host name.
func (s *scan) consumeURL(u, party string, ev Evidence) {
	for _, v := range vendors {
		if vendorPatterns[v.name].MatchString(u) {
			s.consumeVendor(v.name, party, medium, ev)
			return
		}
	}
	if m := serviceURL.FindStringSubmatch(`"` + u + `"`); m != nil {
		s.consumeNamed("http", m[1], party, ev)
	}
}

// resolveCall matches a call to the one other component that serves its
// path.
func (s *scan) resolveCall(c pathCall) {
	segs := strings.Split(strings.TrimPrefix(c.path, "/"), "/")
	if genericPaths[segs[0]] || strings.HasPrefix(segs[0], ":") {
		return
	}
	servers := map[string]bool{}
	for _, r := range s.routes {
		route := strings.Split(strings.TrimPrefix(r.path, "/"), "/")
		// A route starting with a parameter is a sub-router's, mounted at a
		// path read elsewhere, so it can't be matched.
		if r.comp != c.comp && !param(route[0]) && pathsMatch(route, segs) {
			servers[r.comp] = true
		}
	}
	if len(servers) == 1 {
		for comp := range servers {
			s.consume("http", comp, generic, c.comp, high, c.ev)
		}
	}
}

// pathsMatch says whether a call's path can reach a route: segment by
// segment, where a parameter on either side matches anything, and a call
// may leave off the route's trailing parameters.
func pathsMatch(route, call []string) bool {
	if len(call) > len(route) {
		return false
	}
	for i, seg := range route {
		switch {
		case i >= len(call):
			if !param(seg) {
				return false
			}
		case seg != call[i] && !param(seg) && !param(call[i]):
			return false
		}
	}
	return true
}

// param says whether a path segment is a parameter: :id, {id}, <id> or *.
func param(seg string) bool { return seg != "" && strings.ContainsRune(":{<*", rune(seg[0])) }
