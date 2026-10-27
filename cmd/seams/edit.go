package main

import (
	"errors"
	"flag"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/bitshepherds/sheepdog-seams/internal/registry"
)

// names is a flag that may be given more than once, or as a comma list.
type names []string

func (n *names) String() string { return strings.Join(*n, ",") }

func (n *names) Set(v string) error {
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			*n = append(*n, s)
		}
	}
	return nil
}

// details are the flags that describe a seam, shared by add and update.
type details struct {
	audience, contract, owner, independent, tool string
	external, inRepo, notInRepo, tests           names
}

func (d *details) on(fs *flag.FlagSet) {
	fs.StringVar(&d.audience, "audience", "", "internal, with its consumers named, or public")
	fs.StringVar(&d.contract, "contract", "", "where the contract lives: a path in the repo, or a URL")
	fs.StringVar(&d.owner, "owner", "", "who owns the contract")
	fs.StringVar(&d.independent, "deploys-independently", "", "true or false: whether the sides deploy separately")
	fs.Var(&d.external, "external", "a party outside the system, such as a vendor (repeatable)")
	fs.Var(&d.inRepo, "in-repo", "a party whose code is in this repo (repeatable)")
	fs.Var(&d.notInRepo, "not-in-repo", "a party whose code is in another repo, such as a pinned tool (repeatable)")
	fs.Var(&d.tests, "test", "a contract test, as <provider|consumer>:<test>:<tier> (repeatable)")
	fs.StringVar(&d.tool, "tool", "", "the tool that runs the --test(s) given with it, such as Pact")
}

// apply sets on the seam each detail given.
func (d *details) apply(s *registry.Seam) error {
	if d.audience != "" {
		if d.audience != registry.Internal && d.audience != registry.Public {
			return fmt.Errorf("--audience is internal or public, not %q", d.audience)
		}
		s.Audience = d.audience
	}
	if d.contract != "" {
		// Moving a contract keeps its owner, unless --owner names a new one.
		if s.Contract == nil {
			s.Contract = &registry.Contract{}
		}
		s.Contract.Location = d.contract
	}
	if d.owner != "" {
		if s.Contract == nil {
			return errors.New("--owner needs a contract: give --contract too")
		}
		s.Contract.Owner = d.owner
	}
	if d.independent != "" {
		v, err := strconv.ParseBool(d.independent)
		if err != nil {
			return fmt.Errorf("--deploys-independently is true or false, not %q", d.independent)
		}
		s.DeploysIndependently = &v
	}
	if d.tool != "" && len(d.tests) == 0 {
		return errors.New("--tool names the tool that runs a --test: give --test too")
	}
	for _, spec := range d.tests {
		test, err := contractTest(spec)
		if err != nil {
			return err
		}
		test.Tool = d.tool
		s.TestWorld = append(s.TestWorld, test)
	}
	for _, p := range parties(s) {
		p.External = p.External || slices.Contains(d.external, p.Name)
	}
	return d.place(s)
}

// place records each party --in-repo names as in this repo, and each
// --not-in-repo names as not, replacing what it had. It refuses a party named
// by both, or one the seam doesn't have, so a typo isn't ignored.
func (d *details) place(s *registry.Seam) error {
	for _, name := range d.inRepo {
		if slices.Contains(d.notInRepo, name) {
			return fmt.Errorf("%s can't be both in this repo and not: give --in-repo or --not-in-repo, not both", name)
		}
	}
	ps := parties(s)
	for _, name := range slices.Concat(d.inRepo, d.notInRepo) {
		if !slices.ContainsFunc(ps, func(p *registry.Party) bool { return p.Name == name }) {
			return fmt.Errorf("%s isn't a party to %s, so it can't be recorded as in this repo or not", name, s.ID)
		}
	}
	for _, p := range ps {
		switch {
		case slices.Contains(d.inRepo, p.Name):
			p.InRepo = registry.Yes()
		case slices.Contains(d.notInRepo, p.Name):
			p.InRepo = registry.No()
		}
	}
	return nil
}

// contractTest reads <side>:<test>:<tier>, where the test may hold colons.
func contractTest(spec string) (registry.ContractTest, error) {
	first, last := strings.Index(spec, ":"), strings.LastIndex(spec, ":")
	if first < 0 || first == last {
		return registry.ContractTest{}, fmt.Errorf("--test is <provider|consumer>:<test>:<tier>, not %q", spec)
	}
	test := registry.ContractTest{Side: spec[:first], Test: spec[first+1 : last], Tier: spec[last+1:]}
	if test.Side != "provider" && test.Side != "consumer" {
		return registry.ContractTest{}, fmt.Errorf("--test's side is provider or consumer, not %q", test.Side)
	}
	return test, nil
}

// parties are pointers to the seam's provider, if it has one, and consumers.
func parties(s *registry.Seam) []*registry.Party {
	var ps []*registry.Party
	if s.Provider != nil {
		ps = append(ps, s.Provider)
	}
	for i := range s.Consumers {
		ps = append(ps, &s.Consumers[i])
	}
	return ps
}

// sides resolves --provider and --consumer, or --sender and --receiver, to a
// seam's provider and consumers. An event's sender is its provider, since the
// event is a fact the sender defines; a command queue's receiver is, since it
// defines the messages it accepts.
func sides(kind, provider string, consumers, senders, receivers names) (string, names, error) {
	if len(senders)+len(receivers) == 0 {
		return provider, consumers, nil
	}
	if provider != "" || len(consumers) > 0 {
		return "", nil, errors.New("give --provider and --consumer, or --sender and --receiver, not both")
	}
	one := func(flag string, ns names) (string, error) {
		if len(ns) != 1 {
			return "", fmt.Errorf("a %s seam has one provider, so give one %s", kind, flag)
		}
		return ns[0], nil
	}
	switch kind {
	case "event":
		p, err := one("--sender", senders)
		return p, receivers, err
	case "queue":
		p, err := one("--receiver", receivers)
		return p, senders, err
	}
	return "", nil, fmt.Errorf("--sender and --receiver are for event and queue seams; a %s seam takes --provider and --consumer", kind)
}

func add(c *cli, args []string) int {
	fs := c.flags("add")
	var provider string
	var consumers, senders, receivers names
	var d details
	fs.StringVar(&provider, "provider", "", "who owns the exchange surface; the id's provider by default, none when it's _")
	fs.Var(&consumers, "consumer", "a consumer (repeatable)")
	fs.Var(&senders, "sender", "for an event or a queue: who sends (repeatable)")
	fs.Var(&receivers, "receiver", "for an event or a queue: who receives (repeatable)")
	d.on(fs)
	if !c.parse(fs, args, 1) {
		return 2
	}
	id, err := registry.ParseID(c.args[0])
	if err != nil {
		return c.fail(err)
	}
	provider, consumers, err = sides(id.Kind, provider, consumers, senders, receivers)
	if err != nil {
		return c.fail(err)
	}
	s := registry.Seam{ID: id.String(), Audience: registry.Internal}
	switch {
	case provider != "":
		s.Provider = &registry.Party{Name: provider}
	case id.Provider != registry.NoProvider:
		s.Provider = &registry.Party{Name: id.Provider}
	}
	for _, name := range consumers {
		s.Consumers = append(s.Consumers, registry.Party{Name: name})
	}
	if len(s.Consumers) == 0 {
		return c.fail(errors.New("a seam has at least one consumer: give --consumer, or --sender or --receiver"))
	}
	return c.edit(func(r *registry.Registry) (*registry.Seam, error) {
		if err := d.apply(&s); err != nil {
			return nil, err
		}
		if err := r.Add(s); err != nil {
			return nil, err
		}
		added, _ := r.Find(s.ID)
		return added, nil
	}, "added")
}

func update(c *cli, args []string) int {
	fs := c.flags("update")
	var rename string
	var adding, removing names
	var d details
	fs.StringVar(&rename, "rename", "", "a new id; the old one is kept under was")
	fs.Var(&adding, "add-consumer", "a consumer to add (repeatable)")
	fs.Var(&removing, "remove-consumer", "a consumer to remove (repeatable)")
	d.on(fs)
	if !c.parse(fs, args, 1) {
		return 2
	}
	return c.edit(func(r *registry.Registry) (*registry.Seam, error) {
		var detailErr error
		s, err := r.Update(c.args[0], func(s *registry.Seam) {
			if rename != "" {
				registry.Rename(s, rename)
			}
			s.Consumers = slices.DeleteFunc(s.Consumers, func(p registry.Party) bool { return slices.Contains(removing, p.Name) })
			for _, name := range adding {
				s.Consumers = append(s.Consumers, registry.Party{Name: name})
			}
			detailErr = d.apply(s)
		})
		if detailErr != nil {
			return nil, detailErr
		}
		return s, err
	}, "updated")
}

// edit loads the registry, changes it, and saves it, then writes the seam it
// changed.
func (c *cli) edit(change func(*registry.Registry) (*registry.Seam, error), done string) int {
	r, err := c.load()
	if err != nil {
		return c.fail(err)
	}
	s, err := change(r)
	if err != nil {
		return c.fail(err)
	}
	if err := r.Save(c.registry); err != nil {
		return c.fail(err)
	}
	if c.json {
		c.emit("seam", map[string]any{"seam": s})
	} else {
		_, _ = fmt.Fprintln(c.out, done, s.ID)
	}
	return 0
}

func reject(c *cli, args []string) int {
	fs := c.flags("reject")
	var rj registry.Rejection
	fs.StringVar(&rj.Signal, "signal", "", "the kind of evidence that proposed it (required)")
	fs.StringVar(&rj.File, "file", "", "the evidence's file (required)")
	fs.StringVar(&rj.Reason, "reason", "", "why it isn't a seam")
	if !c.parse(fs, args, 1) {
		return 2
	}
	rj.ID = c.args[0]
	if rj.Signal == "" || rj.File == "" {
		return c.fail(errors.New("a rejection names the evidence it rejects: give --signal and --file"))
	}
	r, err := c.load()
	if err != nil {
		return c.fail(err)
	}
	added, err := r.Reject(rj)
	if err != nil {
		return c.fail(err)
	}
	if err := r.Save(c.registry); err != nil {
		return c.fail(err)
	}
	if c.json {
		c.emit("rejection", map[string]any{"rejection": rj, "new": added})
	} else if added {
		_, _ = fmt.Fprintln(c.out, "rejected", rj.ID)
	} else {
		_, _ = fmt.Fprintln(c.out, "rejected already:", rj.ID)
	}
	return 0
}
