// Command seams finds a system's contract boundaries, and keeps a registry of
// each one's provider, consumers, contract and test world.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/bitshepherds/seams/internal/registry"
	"github.com/bitshepherds/seams/schemas"
)

// version is stamped in at build, by the release: -ldflags "-X main.version=…".
var version = "dev"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) } // coverage-ignore: os.Exit ends the test run; the scenarios cover it

// A command runs with its own arguments, and returns the exit code.
type command struct {
	summary string
	run     func(c *cli, args []string) int
}

var commands = map[string]command{
	"add":      {"register a seam", add},
	"detect":   {"propose the repo's seams, with evidence; --drift compares them with the registry", detectSeams},
	"import":   {"fill in seams from catalog-info.yaml", importFrom},
	"list":     {"list the registered seams, one a line", list},
	"reject":   {"remember a candidate that isn't a seam, so it isn't proposed again", reject},
	"render":   {"write the register, with a diagram, to docs/SEAMS.md", render},
	"show":     {"show one seam, or with no id the register of them all", show},
	"update":   {"change a registered seam", update},
	"validate": {"check the registry, and report its findings", validate},
}

// cli is what every command shares: where it writes, and the common flags.
type cli struct {
	out, errs io.Writer
	registry  string
	root      string // the repo whose folders are checked: detect's --root, else the working directory
	json      bool
	args      []string // the command's positional arguments
}

func run(args []string, out, errs io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		_, _ = fmt.Fprintln(out, "seams", version)
		return 0
	}
	if len(args) == 0 {
		usage(errs)
		return 2
	}
	cmd, ok := commands[args[0]]
	if !ok {
		_, _ = fmt.Fprintf(errs, "seams: no command %q\n", args[0])
		usage(errs)
		return 2
	}
	return cmd.run(&cli{out: out, errs: errs, root: "."}, args[1:])
}

func usage(w io.Writer) {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	_, _ = fmt.Fprintln(w, "usage: seams <command> [--registry <file>] [--json] ..., or seams --version")
	for _, name := range names {
		_, _ = fmt.Fprintf(w, "  %-10s %s\n", name, commands[name].summary)
	}
}

// flags is a command's flag set, with --registry and --json already on it.
func (c *cli) flags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet("seams "+name, flag.ContinueOnError)
	fs.SetOutput(c.errs)
	fs.StringVar(&c.registry, "registry", registry.DefaultPath, "the registry file")
	fs.BoolVar(&c.json, "json", false, "write JSON, matching the command's published schema")
	return fs
}

// parse reads a command's flags, before or after its positional arguments,
// into c.args, and says whether to carry on; on false, the command returns 2.
func (c *cli) parse(fs *flag.FlagSet, args []string, positional int) bool {
	return c.parseUpTo(fs, args, positional, positional)
}

// parseUpTo is parse for a command that takes between least and most
// positional arguments.
func (c *cli) parseUpTo(fs *flag.FlagSet, args []string, least, most int) bool {
	c.args = nil
	for {
		if err := fs.Parse(args); err != nil {
			return false
		}
		if fs.NArg() == 0 {
			break
		}
		c.args = append(c.args, fs.Arg(0))
		args = fs.Args()[1:]
	}
	if len(c.args) < least || len(c.args) > most {
		want := strconv.Itoa(least)
		if most > least {
			want += " to " + strconv.Itoa(most)
		}
		_, _ = fmt.Fprintf(c.errs, "%s: want %s argument(s), got %q\n", fs.Name(), want, strings.Join(c.args, " "))
		return false
	}
	return true
}

// fail reports an error, and returns the exit code for it.
func (c *cli) fail(err error) int {
	_, _ = fmt.Fprintln(c.errs, "seams:", err)
	return 1
}

// emit writes a document to its schema, with the schema's $id in it.
func (c *cli) emit(schema string, doc map[string]any) {
	doc["schema"] = schemas.ID(schema)
	data, _ := json.MarshalIndent(doc, "", "  ") // the documents hold only plain values
	_, _ = fmt.Fprintln(c.out, string(data))
}

// load reads the registry, once the repo's folders pass registry.CheckFolder.
// Every command that reads or writes seams files loads first, after its
// flags, so --help never meets the check.
func (c *cli) load() (*registry.Registry, error) {
	if err := registry.CheckFolder(c.root); err != nil {
		return nil, err
	}
	return registry.Load(c.registry)
}

var errProblems = errors.New("the registry has problems")

func validate(c *cli, args []string) int {
	fs := c.flags("validate")
	if !c.parse(fs, args, 0) {
		return 2
	}
	r, err := c.load()
	if err != nil {
		return c.fail(err)
	}
	rep := r.Validate()
	if c.json {
		c.emit("validate", map[string]any{"ok": rep.OK(), "problems": rep.Problems, "findings": rep.Findings})
	} else {
		for _, n := range rep.Problems {
			_, _ = fmt.Fprintf(c.out, "problem: %s %s\n", n.Seam, n.Message)
		}
		for _, n := range rep.Findings {
			_, _ = fmt.Fprintf(c.out, "finding: %s %s\n", n.Seam, n.Message)
		}
		_, _ = fmt.Fprintf(c.out, "%d seam(s), %d problem(s), %d finding(s)\n", len(r.Seams), len(rep.Problems), len(rep.Findings))
	}
	if !rep.OK() {
		return c.fail(errProblems)
	}
	return 0
}
