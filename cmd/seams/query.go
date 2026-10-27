package main

import (
	"fmt"
	"strings"

	"github.com/bitshepherds/sheepdog-seams/internal/register"
	"github.com/bitshepherds/sheepdog-seams/internal/registry"
)

func list(c *cli, args []string) int {
	fs := c.flags("list")
	if !c.parse(fs, args, 0) {
		return 2
	}
	r, err := c.load()
	if err != nil {
		return c.fail(err)
	}
	if c.json {
		c.emit("list", map[string]any{"seams": r.Seams})
		return 0
	}
	for _, s := range r.Seams {
		_, _ = fmt.Fprintf(c.out, "%s  %s → %s (%s)\n", s.ID, register.ProviderName(s), register.ConsumerNames(s), s.Audience)
	}
	return 0
}

func show(c *cli, args []string) int {
	fs := c.flags("show")
	statusPath := fs.String("status", "", "a status file, rating each seam on the coverage ladder")
	if !c.parseUpTo(fs, args, 0, 1) {
		return 2
	}
	r, err := c.load()
	if err != nil {
		return c.fail(err)
	}
	st, err := c.loadStatus(*statusPath, r)
	if err != nil {
		return c.fail(err)
	}
	if len(c.args) == 0 {
		return c.showRegister(register.Build(r, st), st != nil)
	}
	s, ok := r.Find(c.args[0])
	if !ok {
		return c.fail(fmt.Errorf("no seam %s", c.args[0]))
	}
	e := register.Rate(r, *s, st)
	if c.json {
		doc := map[string]any{"seam": s, "top": e.Top}
		if e.Rung != nil {
			doc["rung"] = *e.Rung
		}
		if len(e.Gaps) > 0 {
			doc["gaps"] = e.Gaps
		}
		c.emit("seam", doc)
		return 0
	}
	c.describe(e, st != nil)
	return 0
}

// loadStatus reads the status file at path, or none when path is empty,
// and warns of each rating for a seam the registry doesn't hold.
func (c *cli) loadStatus(path string, r *registry.Registry) (*register.Status, error) {
	if path == "" {
		return nil, nil
	}
	st, err := register.LoadStatus(path)
	if err != nil {
		return nil, err
	}
	for _, id := range st.Unknown(r) {
		_, _ = fmt.Fprintf(c.errs, "seams: %s rates %s, which isn't registered\n", path, id)
	}
	return st, nil
}

func (c *cli) showRegister(entries []register.Entry, rated bool) int {
	if c.json {
		c.emit("register", map[string]any{"seams": entries})
		return 0
	}
	out := 0
	for i, e := range entries {
		if i > 0 {
			_, _ = fmt.Fprintln(c.out)
		}
		c.describe(e, rated)
		if e.CrossesOut {
			out++
		}
	}
	_, _ = fmt.Fprintf(c.out, "\n%d seam(s): %d internal, %d crossing out of the system\n", len(entries), len(entries)-out, out)
	return 0
}

// describe prints a seam's characteristics, one a line, and its rung when
// a status file was given.
func (c *cli) describe(e register.Entry, rated bool) {
	s := e.Seam
	line := func(label, value string) { _, _ = fmt.Fprintf(c.out, "%-22s %s\n", label+":", value) }
	_, _ = fmt.Fprintln(c.out, s.ID)
	if len(s.Was) > 0 {
		line("was", strings.Join(s.Was, ", "))
	}
	if rated {
		line("rung", register.RungOf(e))
		if len(e.Gaps) > 0 {
			line("gaps", strings.Join(e.Gaps, "; "))
		}
	}
	line("kind", e.Kind)
	line("provider", register.ProviderName(s))
	line("consumers", register.ConsumerNames(s))
	line("audience", s.Audience)
	boundary := "internal"
	if e.CrossesOut {
		boundary = "crosses out of the system"
	}
	line("boundary", boundary)
	if s.Contract != nil {
		line("contract", register.ContractOf(s))
	}
	if s.DeploysIndependently != nil {
		line("deploys independently", fmt.Sprint(*s.DeploysIndependently))
	}
	for _, t := range register.TestWorld(s) {
		side, test, _ := strings.Cut(t, ": ")
		line(side+" test", test)
	}
}
