package detect

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"
)

// dep is one dependency a manifest declares.
type dep struct {
	name string // as the ecosystem writes it: a Go module path, an npm or PyPI name
	file string
	line int
}

var (
	goModule      = regexp.MustCompile(`(?m)^module\s+(\S+)`)
	goRequire     = regexp.MustCompile(`^\s*(?:require\s+)?([\w.~-]+\.[\w.~/-]+)\s+v\S+`)
	majorVersion  = regexp.MustCompile(`/v\d+$`)
	tomlName      = regexp.MustCompile(`^name\s*=\s*"([^"]+)"`)
	tomlSection   = regexp.MustCompile(`^\[([^\]]+)\]`)
	pyArrayDep    = regexp.MustCompile(`^\s*"([A-Za-z0-9][A-Za-z0-9._-]*)`)
	pyTableDep    = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)\s*=`)
	pyInlineDeps  = regexp.MustCompile(`^dependencies\s*=\s*\[(.*)\]`)
	pyQuoted      = regexp.MustCompile(`"([A-Za-z0-9][A-Za-z0-9._-]*)`)
	pyRequirement = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)`)
	yamlService   = regexp.MustCompile(`^service:\s*['"]?([\w.-]+)`)
)

// manifestName is the name a manifest gives its component, or empty when it
// gives none.
func (s *scan) manifestName(file string) string {
	switch path.Base(file) {
	case "go.mod":
		if m := goModule.FindStringSubmatch(s.read(file)); m != nil {
			return slug(path.Base(majorVersion.ReplaceAllString(m[1], "")))
		}
	case "package.json":
		var pkg struct{ Name string }
		_ = json.Unmarshal([]byte(s.read(file)), &pkg)
		return slug(path.Base(pkg.Name))
	case "pyproject.toml":
		for _, l := range s.lines(file) {
			if m := tomlName.FindStringSubmatch(l); m != nil {
				return slug(m[1])
			}
		}
	default: // serverless.yml
		for _, l := range s.lines(file) {
			if m := yamlService.FindStringSubmatch(l); m != nil {
				return slug(m[1])
			}
		}
	}
	return ""
}

// moduleName is a manifest's full name, such as a Go module's path or an npm
// package's scoped name, which other components depend on it by.
func (s *scan) moduleName(c Component) string {
	switch path.Base(c.Manifest) {
	case "go.mod":
		m := goModule.FindStringSubmatch(s.read(c.Manifest))
		return m[1]
	case "package.json":
		var pkg struct{ Name string }
		_ = json.Unmarshal([]byte(s.read(c.Manifest)), &pkg)
		return pkg.Name
	case "pyproject.toml":
		return pyName(c.Name)
	}
	return ""
}

// pyName is a PyPI name as pip compares it: lower case, with '_' and '.' as '-'.
func pyName(n string) string {
	return strings.NewReplacer("_", "-", ".", "-").Replace(strings.ToLower(n))
}

// readDeps lists every component's dependencies, from its manifest and, for
// Python, its requirements.txt.
func (s *scan) readDeps() {
	for _, c := range s.comps {
		var deps []dep
		switch path.Base(c.Manifest) {
		case "go.mod":
			deps = s.goDeps(c.Manifest)
		case "package.json":
			deps = s.npmDeps(c.Manifest)
		case "pyproject.toml":
			deps = s.pyprojectDeps(c.Manifest)
		}
		if req := path.Join(c.Dir, "requirements.txt"); s.read(req) != "" {
			deps = append(deps, s.requirements(req)...)
		}
		s.deps[c.Name] = append(s.deps[c.Name], deps...)
	}
}

func (s *scan) goDeps(file string) []dep {
	var deps []dep
	for i, l := range s.lines(file) {
		if m := goRequire.FindStringSubmatch(l); m != nil {
			deps = append(deps, dep{m[1], file, i + 1})
		}
	}
	return deps
}

func (s *scan) npmDeps(file string) []dep {
	var pkg struct {
		Dependencies, DevDependencies, PeerDependencies map[string]string
	}
	_ = json.Unmarshal([]byte(s.read(file)), &pkg)
	var deps []dep
	for _, set := range []map[string]string{pkg.Dependencies, pkg.DevDependencies, pkg.PeerDependencies} {
		for name := range set {
			deps = append(deps, dep{name, file, s.lineOf(file, `"`+name+`"`)})
		}
	}
	sort.Slice(deps, func(i, j int) bool { return deps[i].line < deps[j].line })
	return deps
}

// pyprojectDeps reads PEP 621's dependencies array and Poetry's
// dependency tables.
func (s *scan) pyprojectDeps(file string) []dep {
	var deps []dep
	section, inArray := "", false
	for i, l := range s.lines(file) {
		if m := tomlSection.FindStringSubmatch(l); m != nil {
			section, inArray = m[1], false
			continue
		}
		switch {
		case inArray:
			if strings.HasPrefix(strings.TrimSpace(l), "]") {
				inArray = false
			} else if m := pyArrayDep.FindStringSubmatch(l); m != nil {
				deps = append(deps, dep{pyName(m[1]), file, i + 1})
			}
		case section == "project" && pyInlineDeps.MatchString(l):
			for _, m := range pyQuoted.FindAllStringSubmatch(pyInlineDeps.FindStringSubmatch(l)[1], -1) {
				deps = append(deps, dep{pyName(m[1]), file, i + 1})
			}
		case section == "project" && strings.HasPrefix(l, "dependencies"):
			inArray = true
		case strings.HasSuffix(section, "dependencies") && strings.HasPrefix(section, "tool.poetry"):
			if m := pyTableDep.FindStringSubmatch(l); m != nil && m[1] != "python" {
				deps = append(deps, dep{pyName(m[1]), file, i + 1})
			}
		}
	}
	return deps
}

func (s *scan) requirements(file string) []dep {
	var deps []dep
	for i, l := range s.lines(file) {
		if m := pyRequirement.FindStringSubmatch(l); m != nil {
			deps = append(deps, dep{pyName(m[1]), file, i + 1})
		}
	}
	return deps
}

// depends is the component's dependency on any of the named packages: for
// Go, on the module or one under it.
func (s *scan) depends(comp string, names ...string) (dep, bool) {
	for _, d := range s.deps[comp] {
		for _, n := range names {
			if d.name == n || strings.HasPrefix(d.name, n+"/") {
				return d, true
			}
		}
	}
	return dep{}, false
}
