package detect

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// Signals read from code.
const (
	sigHTTPRoute  = "http-route"
	sigHTTPClient = "http-client"
	sigRouteCall  = "route-call"
	sigGRPC       = "grpc"
	sigMessaging  = "messaging"
	sigEnv        = "env"
)

// sourceExts are the languages whose code is read.
var sourceExts = map[string]bool{
	".go": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true, ".ts": true, ".tsx": true, ".py": true,
}

var (
	routePatterns = []*regexp.Regexp{
		regexp.MustCompile(`\.(?:HandleFunc|Handle|GET|POST|PUT|PATCH|DELETE|Get|Post|Put|Patch|Delete|Route)\(\s*"(?:[A-Z]+ )?(/[^"]*)"`),
		regexp.MustCompile("\\b(?:app|router|server|fastify|api|routes)\\.(?:get|post|put|patch|delete|all|route)\\(\\s*[\"'`](/[^\"'`]*)"),
		regexp.MustCompile(`@(?:Controller|Get|Post|Put|Patch|Delete)\(\s*["'](/?[^"']*)["']`),
		regexp.MustCompile(`@\w+\.(?:get|post|put|patch|delete|route|api_route)\(\s*["'](/[^"']*)`),
	}
	djangoRoute  = regexp.MustCompile(`\b(?:re_)?path\(\s*r?["']\^?([\w/<>:-]+)`)
	stdlibServer = regexp.MustCompile(`\bhttp\.(?:ListenAndServe(?:TLS)?|createServer)\(`)

	clientCall = regexp.MustCompile(`(?i)\b(?:fetch|axios(?:\.\w+)?|requests\.\w+|httpx\.\w+|http\.(?:get|post|head|newrequest\w*)|got(?:\.\w+)?|client\.(?:get|post|put|patch|delete))\s*\(`)
	quoted     = regexp.MustCompile("[\"'`]([^\"'`]*)[\"'`]")
	template   = regexp.MustCompile(`\$?\{[^}]*\}`)
	serviceURL = regexp.MustCompile("[\"'`](?:https?|grpc)://([a-z][a-z0-9-]*)(?:\\.(?:svc(?:\\.cluster\\.local)?|internal))?(?::\\d+)?(?:/[^\"'`]*)?[\"'`]")

	envRead = regexp.MustCompile(`\bos\.(?:Getenv|LookupEnv|getenv|environ\.get)\(\s*["']([A-Z][A-Z0-9_]*)["']|\bos\.environ\[\s*["']([A-Z][A-Z0-9_]*)["']|\bprocess\.env\.([A-Z][A-Z0-9_]*)|\bprocess\.env\[\s*["']([A-Z][A-Z0-9_]*)["']`)

	grpcServe = regexp.MustCompile(`\bRegister(\w+)Server\(|\badd_(\w+)Servicer_to_server\(|\.addService\(\s*(?:\w+\.)*(\w+)\.service\b`)
	grpcStub  = regexp.MustCompile(`\bNew(\w+)Client\(|\b(\w+)Stub\(|\bnew\s+(?:\w+\.)+(\w+)\([^)]*credentials`)

	publishCall   = regexp.MustCompile("(?i)\\.(?:publish|produce|send)\\(\\s*[\"'`]([\\w.:/-]+)[\"'`]")
	subscribeCall = regexp.MustCompile("(?i)\\.(?:subscribe|queuesubscribe|chansubscribe|consumepartition)\\(\\s*\\[?\\s*[\"'`]([\\w.:/-]+)[\"'`]")
	topicLiteral  = regexp.MustCompile("\\b[Tt]opics?\\s*[:=]\\s*\\[?\\s*[\"'`]([\\w.-]+)[\"'`]")
	queueSend     = regexp.MustCompile(`\bsendToQueue\(\s*["']([\w.-]+)|\bbasic_publish\([^)]*routing_key\s*=\s*["']([\w.-]+)`)
	queueReceive  = regexp.MustCompile(`\bbasic_consume\([^)]*queue\s*=\s*["']([\w.-]+)|(?i)\.consume\(\s*["']([\w.-]+)`)
	sendWords     = regexp.MustCompile(`(?i)send|publish|produc|writer|putevents|put_events|putrecord`)
	receiveWords  = regexp.MustCompile(`(?i)receive|subscri|consum|reader|poll`)

	tableRefPattern = regexp.MustCompile(`(?i)\b(?:from|join|into|update)\s+["` + "`" + `]?([a-z_][a-z0-9_]*)`)
	importLine      = regexp.MustCompile(`^\s*(?:import|from)\s|\bimport\s|\brequire\(`)
)

// Directions a message can travel, from the party's side.
const (
	unknownDirection = iota
	sending
	receiving
)

// route is a path a component serves.
type route struct {
	comp, path string
	ev         Evidence
}

// pathCall is a path a component calls, to be matched to a route.
type pathCall struct {
	comp, path string
	ev         Evidence
}

// envUse is code reading an env name that may name a seam.
type envUse struct {
	comp, name string
	direction  int
	ev         Evidence
}

// tableRef is code naming a table, to be matched to a migration.
type tableRef struct {
	comp, table string
	ev          Evidence
}

// codeFile is what reading one file's code needs to know.
type codeFile struct {
	path, comp               string
	lines                    []string
	grpc, broker, db, django bool
}

func (s *scan) codeSignals() {
	for _, f := range s.files {
		if !sourceExts[path.Ext(f)] {
			continue
		}
		comp := s.component(f)
		_, grpc := s.depends(comp, grpcLibs...)
		_, broker := s.depends(comp, brokers...)
		_, db := s.depends(comp, dbClients...)
		_, django := s.depends(comp, "django")
		cf := codeFile{path: f, comp: comp, lines: s.lines(f), grpc: grpc, broker: broker, db: db, django: django}
		for i := range cf.lines {
			s.codeLine(cf, i)
		}
	}
}

// codeLine reads one line of code, numbered from 0.
func (s *scan) codeLine(cf codeFile, i int) {
	line := cf.lines[i]
	s.routeLine(cf, i, line)
	s.clientLine(cf, i, line)
	s.envLine(cf, i, line)
	s.vendorLine(cf, i, line)
	if cf.grpc {
		s.grpcLine(cf, i, line)
	}
	if cf.broker {
		s.messageLine(cf, i, line)
	}
	if cf.db && !importLine.MatchString(line) {
		for _, m := range tableRefPattern.FindAllStringSubmatch(line, -1) {
			s.tableRefs = append(s.tableRefs, tableRef{cf.comp, strings.ToLower(m[1]), s.evidence(sigDBClient, consumes, cf.comp, cf.path, i+1)})
		}
	}
}

func (s *scan) routeLine(cf codeFile, i int, line string) {
	serve := func(p string) {
		ev := s.evidence(sigHTTPRoute, provides, cf.comp, cf.path, i+1)
		s.provide("http", cf.comp, generic, medium, ev)
		s.routes = append(s.routes, route{cf.comp, normalPath(p), ev})
	}
	for _, re := range routePatterns {
		if m := re.FindStringSubmatch(line); m != nil {
			serve("/" + strings.TrimPrefix(m[1], "/"))
			return
		}
	}
	if m := djangoRoute.FindStringSubmatch(line); m != nil && cf.django {
		serve("/" + m[1])
		return
	}
	if stdlibServer.MatchString(line) {
		s.provide("http", cf.comp, generic, medium, s.evidence(sigHTTPServer, provides, cf.comp, cf.path, i+1))
	}
}

// clientLine reads a call to a named service, or to a path another
// component may serve.
func (s *scan) clientLine(cf codeFile, i int, line string) {
	if m := serviceURL.FindStringSubmatch(line); m != nil && m[1] != "localhost" {
		provider, ok := s.named(m[1])
		conf := medium
		if !ok {
			provider, conf = slug(m[1]), low
		}
		s.consume("http", provider, generic, cf.comp, conf, s.evidence(sigHTTPClient, consumes, cf.comp, cf.path, i+1))
	}
	loc := clientCall.FindStringIndex(line)
	if loc == nil {
		return
	}
	if m := quoted.FindStringSubmatch(line[loc[1]:]); m != nil {
		if p := callPath(m[1]); p != "" {
			s.calls = append(s.calls, pathCall{cf.comp, p, s.evidence(sigRouteCall, consumes, cf.comp, cf.path, i+1)})
		}
	}
}

// callPath is the path a client call's first literal names, with any
// template expressions as parameters, or empty when it names none.
func callPath(lit string) string {
	lit = template.ReplaceAllString(lit, ":p")
	if i := strings.Index(lit, "://"); i >= 0 {
		rest := lit[i+3:]
		j := strings.IndexByte(rest, '/')
		if j < 0 {
			return ""
		}
		lit = rest[j:]
	}
	lit = strings.TrimPrefix(lit, ":p")
	if !strings.HasPrefix(lit, "/") {
		return ""
	}
	return normalPath(lit)
}

// normalPath drops a path's query and trailing slash.
func normalPath(p string) string {
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if p != "/" {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}

func (s *scan) envLine(cf codeFile, i int, line string) {
	for _, m := range envRead.FindAllStringSubmatch(line, -1) {
		name := strings.Join(m[1:], "")
		s.envs = append(s.envs, envUse{cf.comp, name, direction(cf.lines, i, len(cf.lines)), s.evidence(sigEnv, "", cf.comp, cf.path, i+1)})
	}
}

// direction is which way the code near a line sends messages: the nearest
// line, within window lines, that receives or sends says.
func direction(lines []string, i, window int) int {
	for d := 0; d <= window; d++ {
		for _, j := range []int{i - d, i + d} {
			if j < 0 || j >= len(lines) {
				continue
			}
			switch { // receiving first: client.send(new ReceiveMessageCommand(...)) receives
			case receiveWords.MatchString(lines[j]):
				return receiving
			case sendWords.MatchString(lines[j]):
				return sending
			}
		}
	}
	return unknownDirection
}

func (s *scan) grpcLine(cf codeFile, i int, line string) {
	if m := grpcServe.FindStringSubmatch(line); m != nil {
		s.provide("grpc", cf.comp, kebab(strings.Join(m[1:], "")), medium, s.evidence(sigGRPC, provides, cf.comp, cf.path, i+1))
	}
	if m := grpcStub.FindStringSubmatch(line); m != nil {
		s.consume("grpc", "", kebab(strings.Join(m[1:], "")), cf.comp, medium, s.evidence(sigGRPC, consumes, cf.comp, cf.path, i+1))
	}
}

func (s *scan) messageLine(cf codeFile, i int, line string) {
	switch m := firstMatch(line, queueSend, queueReceive, publishCall, subscribeCall); m.re {
	case queueSend:
		s.message("queue", m.name, sending, cf, i, medium)
	case queueReceive:
		s.message("queue", m.name, receiving, cf, i, medium)
	case publishCall:
		s.message("event", m.name, sending, cf, i, medium)
	case subscribeCall:
		s.message("event", m.name, receiving, cf, i, medium)
	default:
		if t := topicLiteral.FindStringSubmatch(line); t != nil {
			s.message("event", t[1], direction(cf.lines, i, 3), cf, i, medium)
		}
	}
}

// match is the first of several patterns a line matched, and the name its
// first non-empty group captured.
type match struct {
	re   *regexp.Regexp
	name string
}

func firstMatch(line string, res ...*regexp.Regexp) match {
	for _, re := range res {
		if m := re.FindStringSubmatch(line); m != nil {
			return match{re, strings.Join(m[1:], "")}
		}
	}
	return match{}
}

// message records a party sending or receiving a message: an event's
// publisher provides it, and a queue's receiver does.
func (s *scan) message(kind, name string, dir int, cf codeFile, i, conf int) {
	s.messageFact(kind, slug(name), dir, cf.comp, conf, func(role string) Evidence {
		return s.evidence(sigMessaging, role, cf.comp, cf.path, i+1)
	})
}

// messageFact records a message's sender or receiver; evidence makes the
// evidence for the role it takes.
func (s *scan) messageFact(kind, name string, dir int, party string, conf int, evidence func(role string) Evidence) {
	if dir == unknownDirection {
		conf = low
	}
	if kind == "event" && dir == sending || kind == "queue" && dir == receiving {
		s.provide(kind, party, name, conf, evidence(provides))
		return
	}
	s.consume(kind, "", name, party, conf, evidence(consumes))
}

// vendorPatterns match a vendor's SDK being imported, or its API's host.
var vendorPatterns = func() map[string]*regexp.Regexp {
	pats := map[string]*regexp.Regexp{}
	for _, v := range vendors {
		var alts []string
		for _, d := range v.deps {
			q := regexp.QuoteMeta(d)
			if strings.Contains(d, ".") && strings.Contains(d, "/") {
				alts = append(alts, `^\s*(?:import\s+)?(?:[\w.]+\s+)?"`+q+`(?:/[^"]*)?"\s*$`)
			} else {
				alts = append(alts, `(?:\bfrom\s+|\brequire\(\s*|\bimport\(\s*)["']`+q+`(?:/[^"']*)?["']`)
			}
		}
		for _, m := range v.pyModules {
			alts = append(alts, `^\s*(?:import|from)\s+`+regexp.QuoteMeta(m)+`\b`)
		}
		for _, h := range v.hosts {
			alts = append(alts, `https?://(?:[\w-]+\.)*`+regexp.QuoteMeta(h))
		}
		pats[v.name] = regexp.MustCompile(strings.Join(alts, "|"))
	}
	return pats
}()

// vendorWords are the words a line must hold for a vendor's pattern to be
// worth trying: its package and module names, and its hosts.
var vendorWords = func() map[string][]string {
	words := map[string][]string{}
	for _, v := range vendors {
		words[v.name] = append(slices.Clone(v.deps), v.pyModules...)
		for _, h := range v.hosts {
			words[v.name] = append(words[v.name], strings.SplitN(h, "/", 2)[0])
		}
	}
	return words
}()

func (s *scan) vendorLine(cf codeFile, i int, line string) {
	for _, v := range vendors {
		if !slices.ContainsFunc(vendorWords[v.name], func(w string) bool { return strings.Contains(line, w) }) {
			continue
		}
		if vendorPatterns[v.name].MatchString(line) {
			s.consumeVendor(v.name, cf.comp, medium, s.evidence(sigVendor, consumes, cf.comp, cf.path, i+1))
		}
	}
}
