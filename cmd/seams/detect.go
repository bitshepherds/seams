package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitshepherds/seams/internal/detect"
	"github.com/bitshepherds/seams/internal/registry"
)

var errDrift = errors.New("the registry has drifted from the code")

func detectSeams(c *cli, args []string) int {
	fs := c.flags("detect")
	root := fs.String("root", ".", "the repo to read")
	drift := fs.Bool("drift", false, "list what the registry lacks, and what it has that the code no longer does; exit 1 if anything")
	if !c.parse(fs, args, 0) {
		return 2
	}
	abs, _ := filepath.Abs(*root) // the working directory is known, or nothing runs
	c.root = abs
	if !flagSet(fs, "registry") {
		c.registry = filepath.Join(abs, registry.DefaultPath)
	}
	r, err := c.load()
	if err != nil {
		return c.fail(err)
	}
	res, err := detect.Detect(os.DirFS(abs), filepath.Base(abs))
	if err != nil {
		return c.fail(err)
	}
	cands := detect.Proposed(res.Candidates, r)
	if *drift {
		return c.drift(detect.FindDrift(cands, r))
	}
	if c.json {
		c.emit("detect", map[string]any{"components": res.Components, "candidates": cands})
		return 0
	}
	for _, cand := range cands {
		c.printCandidate(cand)
	}
	_, _ = fmt.Fprintf(c.out, "%d candidate(s) in %d component(s)\n", len(cands), len(res.Components))
	return 0
}

// flagSet says whether a flag was given.
func flagSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) { set = set || f.Name == name })
	return set
}

func (c *cli) printCandidate(cand detect.Candidate) {
	provider := "(no single owner)"
	if cand.Provider != nil {
		provider = cand.Provider.Name
	}
	var consumers []string
	for _, p := range cand.Consumers {
		consumers = append(consumers, p.Name)
	}
	if len(consumers) == 0 {
		consumers = []string{"(none seen)"}
	}
	_, _ = fmt.Fprintf(c.out, "%s  %s → %s  (%s, %s)\n", cand.ID, provider, strings.Join(consumers, ", "), cand.Confidence, cand.Audience)
	for _, e := range cand.Evidence {
		_, _ = fmt.Fprintf(c.out, "    %-8s %s:%d  %s  %s\n", e.Role, e.File, e.Line, e.Signal, e.Text)
	}
	for _, n := range cand.Notes {
		_, _ = fmt.Fprintf(c.out, "    note: %s\n", n)
	}
}

func (c *cli) drift(d detect.Drift) int {
	if c.json {
		c.emit("drift", map[string]any{"drifted": d.Drifted(), "missing": d.Missing, "stale": d.Stale})
	} else {
		for _, m := range d.Missing {
			_, _ = fmt.Fprintf(c.out, "missing: %s  %s\n", m.Candidate.ID, m.Why)
		}
		for _, s := range d.Stale {
			_, _ = fmt.Fprintf(c.out, "stale: %s  %s\n", s.Seam, s.Why)
		}
		_, _ = fmt.Fprintf(c.out, "%d missing, %d stale\n", len(d.Missing), len(d.Stale))
	}
	if d.Drifted() {
		return c.fail(errDrift)
	}
	return 0
}
