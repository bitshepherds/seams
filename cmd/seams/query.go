package main

import (
	"fmt"
	"strings"

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
		_, _ = fmt.Fprintf(c.out, "%s  %s → %s (%s)\n", s.ID, providerName(s), consumerNames(s), s.Audience)
	}
	return 0
}

func show(c *cli, args []string) int {
	fs := c.flags("show")
	if !c.parse(fs, args, 1) {
		return 2
	}
	r, err := c.load()
	if err != nil {
		return c.fail(err)
	}
	s, ok := r.Find(c.args[0])
	if !ok {
		return c.fail(fmt.Errorf("no seam %s", c.args[0]))
	}
	if c.json {
		c.emit("seam", map[string]any{"seam": s})
		return 0
	}
	line := func(label, value string) { _, _ = fmt.Fprintf(c.out, "%-22s %s\n", label+":", value) }
	_, _ = fmt.Fprintln(c.out, s.ID)
	if len(s.Was) > 0 {
		line("was", strings.Join(s.Was, ", "))
	}
	line("provider", providerName(*s))
	line("consumers", consumerNames(*s))
	line("audience", s.Audience)
	if s.Contract != nil {
		line("contract", strings.TrimSpace(s.Contract.Location+" "+ownedBy(s.Contract.Owner)))
	}
	if s.DeploysIndependently != nil {
		line("deploys independently", fmt.Sprint(*s.DeploysIndependently))
	}
	for _, t := range s.TestWorld {
		line(t.Side+" test", t.Test+", in the "+t.Tier)
	}
	return 0
}

func ownedBy(owner string) string {
	if owner == "" {
		return ""
	}
	return "(owned by " + owner + ")"
}

func providerName(s registry.Seam) string {
	if s.Provider == nil {
		return "no single owner"
	}
	return partyName(*s.Provider)
}

func consumerNames(s registry.Seam) string {
	ns := make([]string, len(s.Consumers))
	for i, p := range s.Consumers {
		ns[i] = partyName(p)
	}
	return strings.Join(ns, ", ")
}

// partyName is the party's name, marked when it's external or in this repo.
func partyName(p registry.Party) string {
	var marks []string
	if p.External {
		marks = append(marks, "external")
	}
	if p.InRepo {
		marks = append(marks, "in this repo")
	}
	if len(marks) == 0 {
		return p.Name
	}
	return p.Name + " (" + strings.Join(marks, ", ") + ")"
}
