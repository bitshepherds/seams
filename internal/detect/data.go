package detect

import (
	"net/url"
	"path"
	"regexp"
	"strings"
)

// Signals read from data and config.
const (
	sigMigrations   = "migrations"
	sigDBClient     = "db-client"
	sigConfigSchema = "config-schema"
)

var (
	// migrationDirs are the directories migrations live in, by convention.
	migrationDirs = []string{"migrations", "migrate", "alembic", "flyway", "liquibase"}
	createTable   = regexp.MustCompile("(?i)create\\s+table\\s+(?:if\\s+not\\s+exists\\s+)?(?:[\"`]?\\w+[\"`]?\\.)?[\"`]?([a-z_][a-z0-9_]*)|\\bcreate_?[tT]able\\(\\s*[\"']([a-z_][a-z0-9_]*)")
	versionSeg    = regexp.MustCompile(`^(?:v\d+(?:\.\d+)*|latest|draft-\d+)$`)
)

// tableDef is a table a component's migrations create.
type tableDef struct {
	owner string
	ev    Evidence
}

// notConfig are files with a $schema that configure a tool, not the system.
var notConfig = map[string]bool{"package.json": true, "tsconfig.json": true, "renovate.json": true, "biome.json": true}

func (s *scan) dataSignals() {
	for _, f := range s.files {
		if s.inMigrations(f) {
			s.migrations(f)
		}
	}
	for _, f := range s.filesNamed("*.json", "*.yaml", "*.yml") {
		if !s.ci(f) && !notConfig[path.Base(f)] {
			s.configSchema(f)
		}
	}
}

// inMigrations says whether a file is in a migrations directory.
func (s *scan) inMigrations(f string) bool {
	for _, seg := range strings.Split(path.Dir(f), "/") {
		for _, d := range migrationDirs {
			if seg == d {
				return true
			}
		}
	}
	return false
}

// migrations: a component whose migrations create a table owns it.
func (s *scan) migrations(f string) {
	comp := s.component(f)
	for i, l := range s.lines(f) {
		if m := createTable.FindStringSubmatch(l); m != nil {
			table := strings.ToLower(strings.Join(m[1:], ""))
			ev := s.evidence(sigMigrations, provides, comp, f, i+1)
			s.tables[table] = tableDef{comp, ev}
			s.provide("table", comp, table, medium, ev)
		}
	}
}

// configSchema: a config file with a $schema consumes the schema's
// provider's config contract.
func (s *scan) configSchema(f string) {
	ref := scalar(mapGet(s.yamlRoot(f), "$schema"))
	if ref == "" {
		return
	}
	comp := s.component(f)
	ev := s.evidence(sigConfigSchema, consumes, comp, f, s.lineOf(f, "$schema"))
	if u, err := url.Parse(ref); err == nil && u.Scheme != "" {
		if provider, name := schemaOwner(u); provider != "" {
			s.consume("config", provider, name, comp, low, ev)
		}
		return
	}
	if p, ok := s.resolvePath(f, ref); ok {
		s.consume("config", s.component(p), schemaName(path.Base(p)), comp, medium, ev)
	}
}

// schemaOwner names a schema URL's owner and the config it describes: a
// GitHub repo, or the domain's own name, such as acme for
// schemas.acme.dev. A tool's own schema has none.
func schemaOwner(u *url.URL) (owner, name string) {
	where := u.Host + u.Path
	for _, t := range toolingSchemas {
		if strings.HasPrefix(where, t) || strings.HasSuffix(u.Host, "."+t) {
			return "", ""
		}
	}
	var segs []string
	for _, seg := range strings.Split(u.Path, "/") {
		if seg != "" && !versionSeg.MatchString(seg) {
			segs = append(segs, seg)
		}
	}
	if len(segs) == 0 {
		return "", ""
	}
	name = schemaName(segs[len(segs)-1])
	if (u.Host == "github.com" || u.Host == "raw.githubusercontent.com") && len(segs) >= 2 {
		return slug(segs[1]), name
	}
	labels := strings.Split(u.Host, ".")
	return slug(labels[max(len(labels)-2, 0)]), name
}

// schemaName is the config a schema file describes, from its name:
// settings for settings.schema.json.
func schemaName(base string) string {
	base = strings.TrimSuffix(base, path.Ext(base))
	return slug(strings.TrimSuffix(base, ".schema"))
}
