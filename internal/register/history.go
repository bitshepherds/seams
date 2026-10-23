package register

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bitshepherds/sheepdog-seams/schemas"
)

// DefaultHistory is where a repo keeps its register's history.
const DefaultHistory = ".sheepdog/seams-history.json"

// History is the register's count on each day it was recorded.
type History struct {
	Schema string  `json:"schema"`
	Counts []Count `json:"counts"`
}

// Count is how many seams there were on one day, and how many at each rung.
type Count struct {
	Date  string `json:"date"`
	Seams int    `json:"seams"`
	// Rungs[i] is how many seams were at rung i; only with a status file.
	Rungs []int `json:"rungs,omitempty"`
	// Unrated is how many seams the status file didn't rate, given with Rungs.
	Unrated *int `json:"unrated,omitempty"`
}

// CountOf counts the register on the day; rated says a status file was given.
func CountOf(entries []Entry, date string, rated bool) Count {
	c := Count{Date: date, Seams: len(entries)}
	if !rated {
		return c
	}
	c.Rungs, c.Unrated = make([]int, Top+1), new(int)
	for _, e := range entries {
		if e.Rung == nil {
			*c.Unrated++
		} else {
			c.Rungs[*e.Rung]++
		}
	}
	return c
}

// LoadHistory reads the history at path, or an empty one if there's none yet.
func LoadHistory(path string) (*History, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the history's path is the user's to choose
	if errors.Is(err, fs.ErrNotExist) {
		return &History{Schema: schemas.ID("history"), Counts: []Count{}}, nil
	}
	if err != nil {
		return nil, err
	}
	if err := schemas.Validate("history", data); err != nil {
		return nil, fmt.Errorf("%s doesn't match the history schema: %w", path, err)
	}
	var h History
	_ = json.Unmarshal(data, &h) // the schema has already passed it
	return &h, nil
}

// Record adds the count, in place of any already recorded that day.
func (h *History) Record(c Count) {
	if n := len(h.Counts); n > 0 && h.Counts[n-1].Date == c.Date {
		h.Counts[n-1] = c
		return
	}
	h.Counts = append(h.Counts, c)
}

// Save writes the history to path, making its directory if need be.
func (h *History) Save(path string) error {
	data, _ := json.MarshalIndent(h, "", "  ") // a History always marshals
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}
