package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bitshepherds/seams/internal/register"
)

var errStale = errors.New("the register is out of date: run seams render")

// today is the day a count is recorded for, in UTC.
var today = func() string { return time.Now().UTC().Format(time.DateOnly) }

// renderFlags are seams render's own flags.
type renderFlags struct {
	out, status, lastStatus, history string
	record, check                    bool
}

func render(c *cli, args []string) int {
	fs := c.flags("render")
	var f renderFlags
	fs.StringVar(&f.out, "out", register.DefaultDoc, "where to write the register")
	fs.StringVar(&f.status, "status", "", "a status file, rating each seam on the coverage ladder; kept for --check")
	fs.StringVar(&f.lastStatus, "last-status", register.DefaultStatus, "where the status file last given is kept, and read when none is")
	fs.BoolVar(&f.record, "record", false, "record today's count in the history too")
	fs.StringVar(&f.history, "history", register.DefaultHistory, "the register's history")
	fs.BoolVar(&f.check, "check", false, "write nothing; exit 1 if the register is out of date")
	if !c.parse(fs, args, 0) {
		return 2
	}
	if f.check && f.record {
		_, _ = fmt.Fprintln(c.errs, "seams render: --check writes nothing, so it can't --record")
		return 2
	}
	r, err := c.load()
	if err != nil {
		return c.fail(err)
	}
	given := f.status != ""
	if _, err := os.Stat(f.lastStatus); !given && err == nil {
		f.status = f.lastStatus
	}
	st, err := c.loadStatus(f.status, r)
	if err != nil {
		return c.fail(err)
	}
	h, err := register.LoadHistory(f.history)
	if err != nil {
		return c.fail(err)
	}
	entries := register.Build(r, st)
	if f.record {
		h.Record(register.CountOf(entries, today(), st != nil))
	}
	doc := register.Markdown(entries, h, st != nil)
	if f.check {
		if was, _ := os.ReadFile(f.out); string(was) != doc { //nolint:gosec // the register's path is the user's to choose
			return c.fail(fmt.Errorf("%s: %w", f.out, errStale))
		}
		return 0
	}
	if !given {
		st = nil // nothing new to keep
	}
	if err := f.write(doc, h, st); err != nil {
		return c.fail(err)
	}
	_, _ = fmt.Fprintf(c.out, "wrote %s: %d seam(s)\n", f.out, len(entries))
	return 0
}

// write writes the register, then what it was drawn from, kept to match it:
// the history, when recording, and the status file, when one was given.
func (f renderFlags) write(doc string, h *register.History, st *register.Status) error {
	if err := os.MkdirAll(filepath.Dir(f.out), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(f.out, []byte(doc), 0o600); err != nil {
		return err
	}
	if f.record {
		if err := h.Save(f.history); err != nil {
			return err
		}
	}
	if st != nil {
		return st.Save(f.lastStatus)
	}
	return nil
}
