package detect

import (
	"bytes"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/bitshepherds/seams/internal/registry"
)

// scan is one detection's state: the repo's files and components, and the
// facts read from them so far.
type scan struct {
	fsys  fs.FS
	root  string
	files []string // every file read, as slash paths from the root
	text  map[string]string
	split map[string][]string // text, as lines
	comps []Component
	deps  map[string][]dep // component name → its manifests' dependencies

	facts    []fact
	supports []support

	routes    []route
	calls     []pathCall
	envs      []envUse
	envMap    map[string]map[string]envTarget // component → env name → what IaC sets it to
	tables    map[string]tableDef             // table → its owner's migration
	tableRefs []tableRef
	exports   map[string]string // CloudFormation export → the component exporting it
	imports   []imported        // Fn::ImportValue, resolved once every export is known
}

// skipDirs are directories that hold no seams of the repo's own: vendored
// code, virtual environments, build output, and tests and QA tooling.
var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, "bower_components": true, "third_party": true,
	"venv": true, "__pycache__": true, "site-packages": true,
	"dist": true, "build": true, "out": true, "target": true, "bin": true, "coverage": true,
	"cdk.out": true, "testdata": true, "test": true, "tests": true, "__tests__": true,
	"e2e": true, "cypress": true, "playwright": true, "fixtures": true,
}

// skipFile says whether a file is a test, or generated, by its name.
func skipFile(name string) bool {
	for _, suffix := range []string{"_test.go", "_test.py", ".min.js", ".pb.go", "_pb2.py", "_pb2_grpc.py"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") ||
		strings.HasPrefix(name, "test_") || name == "conftest.py"
}

// maxFile is the largest file read; bigger ones are data, not code.
const maxFile = 1 << 20

func newScan(fsys fs.FS, root string) (*scan, error) {
	s := &scan{fsys: fsys, root: slug(root), text: map[string]string{}, split: map[string][]string{}, deps: map[string][]dep{},
		envMap: map[string]map[string]envTarget{}, tables: map[string]tableDef{}, exports: map[string]string{}}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && p != "." && s.skipDir(p, d.Name()):
			return fs.SkipDir
		case !d.IsDir() && !skipFile(d.Name()):
			s.files = append(s.files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.findComponents()
	return s, nil
}

// skipDir says whether to leave a directory out: one skipDirs names, a
// virtual environment, or a hidden one other than .github's workflows, which
// say what CI checks.
func (s *scan) skipDir(p, name string) bool {
	if p == ".github" || p == ".github/workflows" {
		return false
	}
	if _, err := fs.Stat(s.fsys, path.Join(p, "pyvenv.cfg")); err == nil {
		return true
	}
	return skipDirs[name] || strings.HasPrefix(name, ".")
}

// read is a file's text, or empty for one that's too big, binary or
// unreadable.
func (s *scan) read(p string) string {
	if t, ok := s.text[p]; ok {
		return t
	}
	data, err := fs.ReadFile(s.fsys, p)
	if err != nil || len(data) > maxFile || bytes.IndexByte(data, 0) >= 0 {
		data = nil
	}
	s.text[p] = string(data)
	return s.text[p]
}

// lines is a file's lines.
func (s *scan) lines(p string) []string {
	if ls, ok := s.split[p]; ok {
		return ls
	}
	s.split[p] = strings.Split(s.read(p), "\n")
	return s.split[p]
}

// evidence is the evidence at a file's line, numbered from 1.
func (s *scan) evidence(signal, role, party, file string, line int) Evidence {
	text := ""
	if ls := s.lines(file); line >= 1 && line <= len(ls) {
		text = strings.TrimSpace(ls[line-1])
	}
	if len(text) > 120 {
		text = text[:117] + "..."
	}
	return Evidence{Signal: signal, Role: role, Party: party, File: file, Line: line, Text: text}
}

// lineOf is the number of the first line of a file holding sub, or 1.
func (s *scan) lineOf(file, sub string) int {
	for i, l := range s.lines(file) {
		if strings.Contains(l, sub) {
			return i + 1
		}
	}
	return 1
}

// manifestNames are the files that name a component, in the order tried.
var manifestNames = []string{"go.mod", "package.json", "pyproject.toml", "serverless.yml", "serverless.yaml"}

// findComponents names each directory with a manifest, then each top-level
// directory holding files outside them.
func (s *scan) findComponents() {
	has := map[string]bool{}
	for _, f := range s.files {
		has[f] = true
	}
	dirs := map[string]bool{}
	for _, f := range s.files {
		dirs[path.Dir(f)] = true
	}
	for dir := range dirs {
		for _, m := range manifestNames {
			file := path.Join(dir, m)
			if name := s.manifestName(file); has[file] && name != "" {
				s.comps = append(s.comps, Component{Name: name, Dir: dir, Manifest: file})
				break
			}
		}
	}
	for _, f := range s.files {
		if _, ok := s.componentAt(f); !ok {
			s.comps = append(s.comps, Component{Name: s.dirName(f), Dir: topDir(f)})
		}
	}
	sort.Slice(s.comps, func(i, j int) bool { return s.comps[i].Dir < s.comps[j].Dir })
}

// topDir is the top-level directory a file is in, or "." for one at the root.
func topDir(f string) string {
	if i := strings.IndexByte(f, '/'); i >= 0 {
		return f[:i]
	}
	return "."
}

// dirName names a component with no manifest by its directory.
func (s *scan) dirName(f string) string {
	if dir := topDir(f); dir != "." {
		return slug(dir)
	}
	return s.root
}

// componentAt is the component whose directory holds the file most closely.
func (s *scan) componentAt(f string) (Component, bool) {
	best, found := Component{}, false
	for _, c := range s.comps {
		if within(f, c.Dir) && (!found || len(c.Dir) > len(best.Dir)) {
			best, found = c, true
		}
	}
	return best, found
}

// within says whether a file is in a directory, at any depth.
func within(f, dir string) bool { return dir == "." || strings.HasPrefix(f, dir+"/") }

// component names the component a file belongs to.
func (s *scan) component(f string) string {
	c, _ := s.componentAt(f)
	return c.Name
}

// named is the component a name refers to, such as pricing for
// PRICING_URL, allowing for a -service, -svc or -api suffix on either side.
func (s *scan) named(name string) (string, bool) {
	trim := func(n string) string {
		for _, suffix := range []string{"-service", "-svc", "-api", "-server"} {
			n = strings.TrimSuffix(n, suffix)
		}
		return n
	}
	for _, c := range s.comps {
		if c.Name == name || trim(c.Name) == trim(name) {
			return c.Name, true
		}
	}
	return "", false
}

// inRepo records a party that is one of the repo's components as in this
// repo, and leaves any other unrecorded: detection never says a party isn't.
func (s *scan) inRepo(party string) *bool {
	for _, c := range s.comps {
		if c.Name == party {
			return registry.Yes()
		}
	}
	return nil
}

// filesNamed lists the files whose name matches a glob, such as *.tf.
func (s *scan) filesNamed(globs ...string) []string {
	var out []string
	for _, f := range s.files {
		for _, g := range globs {
			if ok, _ := path.Match(g, path.Base(f)); ok {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// resolvePath is a path a file refers to, relative to its directory, as a
// path from the root, and whether the repo has it.
func (s *scan) resolvePath(from, ref string) (string, bool) {
	p := path.Clean(path.Join(path.Dir(from), ref))
	for _, f := range s.files {
		if f == p {
			return p, true
		}
	}
	return p, false
}
